//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package ecs

import (
	"context"
	"fmt"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

// The unique ID for this builder
const (
	BuilderId           string = "huawei.huaweicloud"
	SystemImageType            = "system"      // system image only
	DataImageType              = "data-disk"   // data disk images only
	SystemDataImageType        = "system-data" // system image and data disk images
	FullImageType              = "full-ecs"    // Full-ECS image, need vault_id
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	AccessConfig `mapstructure:",squash"`
	ImageConfig  `mapstructure:",squash"`
	RunConfig    `mapstructure:",squash"`

	ctx interpolate.Context
}

type Builder struct {
	config Config
	runner multistep.Runner
}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec {
	return b.config.FlatMapstructure().HCL2Spec()
}

func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	err := config.Decode(&b.config, &config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &b.config.ctx,
	}, raws...)
	if err != nil {
		return nil, nil, err
	}

	// Accumulate any errors
	var errs *packer.MultiError
	errs = packer.MultiErrorAppend(errs, b.config.AccessConfig.Prepare(&b.config.ctx)...)
	errs = packer.MultiErrorAppend(errs, b.config.ImageConfig.Prepare(&b.config.ctx)...)
	errs = packer.MultiErrorAppend(errs, b.config.RunConfig.Prepare(&b.config.ctx)...)

	if newImageType, err := calculateAndValidateImageType(b); err != nil {
		errs = packer.MultiErrorAppend(errs, err)
	} else {
		b.config.ImageType = newImageType
	}

	if errs != nil && len(errs.Errors) > 0 {
		return nil, nil, errs
	}

	// By default, instance name is same as image name
	if b.config.InstanceName == "" {
		b.config.InstanceName = b.config.ImageName
	}

	packer.LogSecretFilter.Set(b.config.AccessKey, b.config.SecretKey)
	return nil, nil, nil
}

func (b *Builder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	region := b.config.Region
	imsClient, err := b.config.HcImsClient(region)
	if err != nil {
		return nil, fmt.Errorf("Error initializing image client: %s", err)
	}

	// Setup the state bag and initial state for the steps
	state := new(multistep.BasicStateBag)
	state.Put("config", &b.config)
	state.Put("hook", hook)
	state.Put("ui", ui)

	// Build the steps
	steps := []multistep.Step{
		&StepLoadAZ{
			AvailabilityZone: b.config.AvailabilityZone,
		},
		&StepLoadFlavor{
			Flavor: b.config.Flavor,
		},
		&StepKeyPair{
			Debug:        b.config.PackerDebug,
			Comm:         &b.config.Comm,
			DebugKeyPath: fmt.Sprintf("ecs_%s.pem", b.config.PackerBuildName),
		},
		&StepSourceImageInfo{
			SourceImage:      b.config.RunConfig.SourceImage,
			SourceImageName:  b.config.RunConfig.SourceImageName,
			SourceImageOpts:  b.config.RunConfig.sourceImageOpts,
			SourceMostRecent: b.config.SourceImageFilters.MostRecent,
		},
		&StepCreateNetwork{
			VpcID:          b.config.VpcID,
			Subnets:        b.config.Subnets,
			SecurityGroups: b.config.SecurityGroups,
		},
		&StepCreatePublicipIP{
			PublicipIP:       b.config.FloatingIP,
			ReuseIPs:         b.config.ReuseIPs,
			EIPType:          b.config.EIPType,
			EIPBandwidthSize: b.config.EIPBandwidthSize,
		},
		&StepRunSourceServer{
			Name:             b.config.InstanceName,
			VpcID:            b.config.VpcID,
			Subnets:          b.config.Subnets,
			SecurityGroups:   b.config.SecurityGroups,
			RootVolumeType:   b.config.VolumeType,
			RootVolumeSize:   b.config.VolumeSize,
			UserData:         b.config.UserData,
			UserDataFile:     b.config.UserDataFile,
			ConfigDrive:      b.config.ConfigDrive,
			InstanceMetadata: b.config.InstanceMetadata,
		},
		&StepAttachVolume{
			DataVolumes: b.config.DataVolumes,
			PrefixName:  b.config.InstanceName,
		},
		&StepGetPassword{
			Debug: b.config.PackerDebug,
			Comm:  &b.config.RunConfig.Comm,
		},
		&communicator.StepConnect{
			Config:    &b.config.RunConfig.Comm,
			Host:      CommHost(b.config.RunConfig.Comm.SSHHost),
			SSHConfig: b.config.RunConfig.Comm.SSHConfigFunc(),
		},
		&commonsteps.StepProvision{},
		&commonsteps.StepCleanupTempKeys{
			Comm: &b.config.RunConfig.Comm,
		},
		&StepStopServer{},
		&stepCreateImage{
			WaitTimeout: b.config.WaitImageReadyTimeout,
		},
		&stepAddImageMembers{},
	}

	// Run!
	b.runner = commonsteps.NewRunner(steps, b.config.PackerConfig, ui)
	b.runner.Run(ctx, state)

	// If there was an error, return that
	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	// If there are no images, then just return
	if _, ok := state.GetOk("image"); !ok {
		return nil, nil
	}

	// Build the artifact and return it
	artifact := &Artifact{
		ImageId:        state.Get("image").(string),
		BuilderIdValue: BuilderId,
		Client:         imsClient,
	}

	return artifact, nil
}

func calculateAndValidateImageType(b *Builder) (string, error) {
	imageType := b.config.ImageType

	// calculate image_type if not specified
	if imageType == "" {
		if len(b.config.DataVolumes) == 0 {
			imageType = SystemImageType
		} else {
			if b.config.Vault == "" {
				imageType = DataImageType
			} else {
				imageType = FullImageType
			}
		}
	} else {
		// validate image_type if specified
		var validTypes = []string{SystemImageType, DataImageType, SystemDataImageType, FullImageType}
		if !isStringInSlice(imageType, validTypes) {
			return imageType, fmt.Errorf("expected 'image_type' to be one of %v, got %s", validTypes, imageType)
		}

		if imageType == FullImageType && b.config.Vault == "" {
			return imageType, fmt.Errorf("vault_id is missing for Full-ECS image")
		}
	}

	return imageType, nil
}

func isStringInSlice(check string, valid []string) bool {
	for _, str := range valid {
		if check == str {
			return true
		}
	}

	return false
}
