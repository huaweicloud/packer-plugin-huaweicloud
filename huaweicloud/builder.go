//go:generate struct-markdown
//go:generate mapstructure-to-hcl2 -type Config

package huaweicloud

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
const BuilderId = "huawei.huaweicloud"

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
	computeClient, err := b.config.computeV2Client()
	if err != nil {
		return nil, fmt.Errorf("Error initializing compute client: %s", err)
	}

	imageClient, err := b.config.imageV2Client()
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
			SourceProperties: b.config.SourceImageFilters.Filters.Properties,
		},
		&StepCreateVolume{
			UseBlockStorageVolume: b.config.UseBlockStorageVolume,
			VolumeName:            b.config.VolumeName,
			VolumeType:            b.config.VolumeType,
		},
		&StepRunSourceServer{
			Name:                  b.config.InstanceName,
			SecurityGroups:        b.config.SecurityGroups,
			Networks:              b.config.Networks,
			Ports:                 b.config.Ports,
			VpcID:                 b.config.VpcID,
			Subnets:               b.config.Subnets,
			UserData:              b.config.UserData,
			UserDataFile:          b.config.UserDataFile,
			ConfigDrive:           b.config.ConfigDrive,
			InstanceMetadata:      b.config.InstanceMetadata,
			UseBlockStorageVolume: b.config.UseBlockStorageVolume,
			ForceDelete:           b.config.ForceDelete,
		},
		&StepGetPassword{
			Debug: b.config.PackerDebug,
			Comm:  &b.config.RunConfig.Comm,
		},
		&StepAllocateIp{
			FloatingIP:       b.config.FloatingIP,
			ReuseIPs:         b.config.ReuseIPs,
			EIPType:          b.config.EIPType,
			EIPBandwidthSize: b.config.EIPBandwidthSize,
		},
		&communicator.StepConnect{
			Config: &b.config.RunConfig.Comm,
			Host: CommHost(
				b.config.RunConfig.Comm.SSHHost,
				computeClient,
				b.config.SSHInterface,
				b.config.SSHIPVersion),
			SSHConfig: b.config.RunConfig.Comm.SSHConfigFunc(),
		},
		&commonsteps.StepProvision{},
		&commonsteps.StepCleanupTempKeys{
			Comm: &b.config.RunConfig.Comm,
		},
		&StepStopServer{},
		&StepDetachVolume{
			UseBlockStorageVolume: false,
		},
		&stepCreateImage{},
		&stepUpdateImageMembers{},
		&stepUpdateImageMinDisk{},
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
		Client:         imageClient,
	}

	return artifact, nil
}
