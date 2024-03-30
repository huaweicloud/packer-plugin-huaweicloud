//go:generate packer-sdc mapstructure-to-hcl2 -type Config
//go:generate packer-sdc struct-markdown

package huaweicloudimport

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"

	ims "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
	ecsbuilder "github.com/huaweicloud/packer-builder-huaweicloud/builder/ecs"
)

const (
	BuilderId = "packer.post-processor.huaweicloud-import"

	ImageFileFormatRAW   = "raw"
	ImageFileFormatVHD   = "zvhd2"
	ImageFileFormatVMDK  = "vmdk"
	ImageFileFormatQCOW2 = "qcow2"
)

var (
	validImageFileFormats = []string{
		ImageFileFormatRAW, ImageFileFormatVHD, ImageFileFormatVMDK, ImageFileFormatQCOW2,
	}
	validImageTypes  = []string{"ECS", "BMS"}
	validImageArches = []string{"x86", "arm"}
)

// Configuration of this post processor
type Config struct {
	common.PackerConfig     `mapstructure:",squash"`
	ecsbuilder.AccessConfig `mapstructure:",squash"`

	// The name of the OBS bucket where the RAW, VHD, VMDK, or qcow2 file will be copied to for import.
	// This bucket **must** exist when the post-processor is run.
	OBSBucket string `mapstructure:"obs_bucket_name" required:"true"`
	// The name of the object key in `obs_bucket_name` where the RAW, VHD, VMDK, or qcow2 file will be copied
	// to import.
	OBSObject string `mapstructure:"obs_object_name" required:"false"`
	// The name of the user-defined image, which contains 1-63 characters and only
	// supports Chinese, English, numbers, '-\_,.:[]'.
	ImageName string `mapstructure:"image_name" required:"true"`
	// The OS version, such as: `CentOS 7.0 64bit`.
	// You may refer to [huaweicloud_api_docs](https://support.huaweicloud.com/intl/en-us/api-ims/ims_03_0910.html) for detail.
	OsVersion string `mapstructure:"image_os_version" required:"true"`
	// The minimum size (GB) of the system disk, the value ranges from 1 to 1024 and must be greater than the size of the image file.
	MinDisk int `mapstructure:"min_disk" required:"true"`
	// The format of the import image , Possible values are: `raw`, `zvhd2`, `vmdk` or `qcow2`.
	Format string `mapstructure:"format" required:"true"`

	// The description of the image.
	ImageDescription string `mapstructure:"image_description" required:"false"`
	// The image type. The value can be ECS or BMS, the default value is ECS.
	ImageType string `mapstructure:"image_type" required:"false"`
	// The tags of the packer image in key/value format.
	ImageTags map[string]string `mapstructure:"image_tags" required:"false"`
	// The image architecture type. The value can be x86 and arm, the default value is x86.
	ImageArchitecture string `mapstructure:"image_architecture" required:"false"`
	// The ID of Enterprise Project in which to create the image.
	// If omitted, the HW_ENTERPRISE_PROJECT_ID environment variable is used.
	EnterpriseProjectId string `mapstructure:"enterprise_project_id" required:"false"`
	// Whether to use the quick import method to import the image. (Default: `false`).
	// Currently, only `raw` and `zvhd2` image files are supported, and the size of an image file cannot exceed 1 TB.
	// You are advised to import image files that are smaller than 128 GB with the common method.
	QuickImport bool `mapstructure:"quick_import" required:"false"`
	// Whether we should skip removing the source image file uploaded to OBS
	// after the import process has completed. Possible values are: `true` to
	// leave it in the OBS bucket, `false` to remove it. (Default: `false`).
	SkipClean bool `mapstructure:"skip_clean" required:"false"`
	// Timeout of creating the image. The timeout string is a possibly signed sequence of
	// decimal numbers, each with optional fraction and a unit suffix, such as "40m", "1.5h" or "2h30m".
	// The default timeout is "30m" which means 30 minutes.
	WaitImageReadyTimeout string `mapstructure:"wait_image_ready_timeout" required:"false"`

	ctx interpolate.Context
}

type PostProcessor struct {
	config Config
}

func (p *PostProcessor) ConfigSpec() hcldec.ObjectSpec {
	return p.config.FlatMapstructure().HCL2Spec()
}

func (p *PostProcessor) Configure(raws ...interface{}) error {
	//conf := p.config
	err := config.Decode(&p.config, &config.DecodeOpts{
		PluginType:         BuilderId,
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{
				"obs_object_name",
			},
		},
	}, raws...)
	if err != nil {
		return err
	}

	// Set defaults
	if p.config.OBSObject == "" {
		p.config.OBSObject = "packer-import-{{timestamp}}." + p.config.Format
	}

	if p.config.EnterpriseProjectId == "" {
		p.config.EnterpriseProjectId = os.Getenv("HW_ENTERPRISE_PROJECT_ID")
	}

	errs := new(packersdk.MultiError)

	// Check and render obs_object_name
	if err = interpolate.Validate(p.config.OBSObject, &p.config.ctx); err != nil {
		errs = packersdk.MultiErrorAppend(
			errs, fmt.Errorf("error parsing obs_object_name template: %s", err))
	}

	// Check we have huaweicloud access variables defined somewhere
	errs = packersdk.MultiErrorAppend(errs, p.config.AccessConfig.Prepare(&p.config.ctx)...)

	if !isStringInSlice(p.config.Format, validImageFileFormats, false) {
		errs = packersdk.MultiErrorAppend(
			errs, fmt.Errorf("expected '%s' to be one of %v, but got %s", "format",
				validImageFileFormats, p.config.Format))
	}

	if !isStringInSlice(p.config.ImageArchitecture, validImageArches, false) {
		errs = packersdk.MultiErrorAppend(
			errs, fmt.Errorf("expected '%s' to be one of %v, but got %s", "image_architecture",
				validImageArches, p.config.ImageArchitecture))
	}

	if !isStringInSlice(p.config.ImageType, validImageTypes, false) {
		errs = packersdk.MultiErrorAppend(
			errs, fmt.Errorf("expected '%s' to be one of %v, but got %s", "image_type",
				validImageTypes, p.config.ImageType))
	}

	// Anything which flagged return back up the stack
	if len(errs.Errors) > 0 {
		return errs
	}

	packersdk.LogSecretFilter.Set(p.config.AccessKey, p.config.SecretKey)
	log.Println(p.config)
	return nil
}

func (p *PostProcessor) PostProcess(ctx context.Context, ui packersdk.Ui, artifact packersdk.Artifact) (packersdk.Artifact, bool, bool, error) {
	var err error

	generatedData := artifact.State("generated_data")
	if generatedData == nil {
		// Make sure it's not a nil map so we can assign to it later.
		generatedData = make(map[string]interface{})
	}
	p.config.ctx.Data = generatedData

	rawTimeout := p.config.WaitImageReadyTimeout
	if rawTimeout == "" {
		rawTimeout = "60m"
	}

	waitTimeout, err := time.ParseDuration(rawTimeout)
	if err != nil {
		log.Printf("[WARN] failed to parse `wait_image_ready_timeout` %s: %s", rawTimeout, err)
		waitTimeout = 60 * time.Minute
	}

	region := p.config.Region
	imsClient, err := p.newIMSClient(region)
	if err != nil {
		return nil, false, false, fmt.Errorf("error initializing image service client: %s", err)
	}

	// Render this key since we didn't in the configure phase
	p.config.OBSObject, err = interpolate.Render(p.config.OBSObject, &p.config.ctx)
	if err != nil {
		return nil, false, false, fmt.Errorf("error rendering obs_object_name template: %s", err)
	}

	ui.Message(fmt.Sprintf("Rendered obs_object_name as %s", p.config.OBSObject))

	ui.Message("Looking for image in artifact")
	// Locate the files output from the builder
	var source string
	for _, path := range artifact.Files() {
		if strings.HasSuffix(path, "."+p.config.Format) {
			source = path
			break
		}
	}

	// Hope we found something useful
	if source == "" {
		return nil, false, false, fmt.Errorf("no %s image file found in artifact from builder", p.config.Format)
	}

	bucketName := p.config.OBSBucket
	keyName := p.config.OBSObject

	obsClient, err := p.newOBSClient(region)
	if err != nil {
		return nil, false, false, fmt.Errorf("error initializing OBS service client: %s", err)
	}

	if err := queryBucket(obsClient, bucketName); err != nil {
		return nil, false, false, fmt.Errorf("failed to query bucket %s: %s", bucketName, err)
	}

	ui.Say(fmt.Sprintf("Waiting for uploading image file %s to OBS %s/%s ...", source, bucketName, keyName))

	// upload file to bucket
	if err := uploadFileToObject(obsClient, bucketName, keyName, source); err != nil {
		return nil, false, false, err
	}

	ui.Say(fmt.Sprintf("Image file %s has been uploaded to OBS %s/%s", source, bucketName, keyName))

	var jobId string
	if p.config.QuickImport {
		jobId, err = p.quickImportImage(imsClient, ui)
	} else {
		jobId, err = p.createImageFromObs(imsClient, ui)
	}

	if err != nil {
		return nil, false, false, fmt.Errorf("failed to import image from OBS %s/%s, %s", bucketName, keyName, err)
	}

	ui.Say(fmt.Sprintf("Waiting for importing image from OBS %s/%s ...", bucketName, keyName))
	imageId, err := waitImageJobSuccess(imsClient, waitTimeout, jobId)
	if err != nil {
		return nil, false, false, fmt.Errorf("error on waiting for importing image %s from OBS %s/%s: %s",
			imageId, bucketName, keyName, err)
	}

	// Add the reported huaweicloud image ID to the artifact list
	ui.Say(fmt.Sprintf("Importing the image ID as %s in region %s completed", imageId, p.config.Region))
	artifact = &ecsbuilder.Artifact{
		ImageId:        imageId,
		BuilderIdValue: BuilderId,
		Client:         imsClient,
	}

	if !p.config.SkipClean {
		ui.Message(fmt.Sprintf("Deleting import source OBS object %s/%s", bucketName, keyName))
		if err = deleteFile(obsClient, bucketName, keyName); err != nil {
			return nil, false, false, fmt.Errorf("failed to delete OBS object %s/%s: %s", bucketName, keyName, err)
		}
	}

	return artifact, false, false, nil
}

func (p *PostProcessor) quickImportImage(client *ims.ImsClient, ui packersdk.Ui) (string, error) {
	conf := p.config
	imageUrl := fmt.Sprintf("%s:%s", conf.OBSBucket, conf.OBSObject)
	requestBody := model.QuickImportImageByFileRequestBody{
		Name:      conf.ImageName,
		ImageUrl:  imageUrl,
		MinDisk:   int32(conf.MinDisk),
		OsVersion: conf.OsVersion,
		ImageTags: buildImageTagsForImport(conf),
	}

	if conf.ImageDescription != "" {
		requestBody.Description = &conf.ImageDescription
	}
	if conf.EnterpriseProjectId != "" {
		requestBody.EnterpriseProjectId = &conf.EnterpriseProjectId
	}
	if conf.ImageArchitecture != "" {
		imageArch := new(model.QuickImportImageByFileRequestBodyArchitecture)
		err := imageArch.UnmarshalJSON([]byte(conf.ImageArchitecture))
		if err == nil {
			requestBody.Architecture = imageArch
		} else {
			ui.Message(fmt.Sprintf("The value of `image_architecture` is invalid: %s", err))
		}
	}
	if conf.ImageType != "" {
		imageType := new(model.QuickImportImageByFileRequestBodyType)
		err := imageType.UnmarshalJSON([]byte(conf.ImageType))
		if err == nil {
			requestBody.Type = imageType
		} else {
			ui.Message(fmt.Sprintf("The value of `image_type` is invalid: %s", err))
		}
	}

	request := model.ImportImageQuickRequest{
		Body: &requestBody,
	}

	response, err := client.ImportImageQuick(&request)
	if err != nil {
		return "", err
	}

	if response.JobId == nil {
		return "", fmt.Errorf("can not get the job from API response")
	}

	return *response.JobId, nil
}

func (p *PostProcessor) createImageFromObs(client *ims.ImsClient, ui packersdk.Ui) (string, error) {
	conf := p.config
	imageUrl := fmt.Sprintf("%s:%s", conf.OBSBucket, conf.OBSObject)
	minDisk := int32(conf.MinDisk)
	requestBody := model.CreateImageRequestBody{
		Name:      conf.ImageName,
		ImageUrl:  &imageUrl,
		MinDisk:   &minDisk,
		OsVersion: &conf.OsVersion,
		ImageTags: buildImageTagsForCreate(conf),
	}

	if conf.ImageDescription != "" {
		requestBody.Description = &conf.ImageDescription
	}
	if conf.EnterpriseProjectId != "" {
		requestBody.EnterpriseProjectId = &conf.EnterpriseProjectId
	}
	if conf.ImageArchitecture != "" {
		imageArch := new(model.CreateImageRequestBodyArchitecture)
		err := imageArch.UnmarshalJSON([]byte(conf.ImageArchitecture))
		if err == nil {
			requestBody.Architecture = imageArch
		} else {
			ui.Message(fmt.Sprintf("The value of `image_architecture` is invalid: %s", err))
		}
	}

	if conf.ImageType != "" {
		imageType := new(model.CreateImageRequestBodyType)
		err := imageType.UnmarshalJSON([]byte(conf.ImageType))
		if err == nil {
			requestBody.Type = imageType
		} else {
			ui.Message(fmt.Sprintf("The value of `image_type` is invalid: %s", err))
		}
	}

	request := model.CreateImageRequest{
		Body: &requestBody,
	}

	response, err := client.CreateImage(&request)
	if err != nil {
		return "", err
	}

	if response.JobId == nil {
		return "", fmt.Errorf("can not get the job from API response")
	}

	return *response.JobId, nil
}

func buildImageTagsForImport(conf Config) *[]model.ResourceTag {
	if len(conf.ImageTags) == 0 {
		return nil
	}

	taglist := make([]model.ResourceTag, len(conf.ImageTags))
	index := 0
	for k, v := range conf.ImageTags {
		taglist[index] = model.ResourceTag{
			Key:   k,
			Value: v,
		}
		index++
	}

	return &taglist
}

func buildImageTagsForCreate(conf Config) *[]model.TagKeyValue {
	if len(conf.ImageTags) == 0 {
		return nil
	}

	taglist := make([]model.TagKeyValue, len(conf.ImageTags))
	index := 0
	for k, v := range conf.ImageTags {
		taglist[index] = model.TagKeyValue{
			Key:   k,
			Value: v,
		}
		index++
	}

	return &taglist
}

func isStringInSlice(key string, valid []string, ignoreCase bool) bool {
	if key == "" {
		return true
	}

	for _, str := range valid {
		if key == str || (ignoreCase && strings.EqualFold(key, str)) {
			return true
		}
	}

	return false
}

func waitImageJobSuccess(client *ims.ImsClient, timeout time.Duration, jobID string) (string, error) {
	stateConf := &ecsbuilder.StateChangeConf{
		Pending:      []string{"INIT", "RUNNING"},
		Target:       []string{"SUCCESS"},
		Refresh:      getImsJobStatus(client, jobID),
		Timeout:      timeout,
		Delay:        60 * time.Second,
		PollInterval: 10 * time.Second,
	}

	result, err := stateConf.WaitForState()
	if err != nil {
		return "", err
	}

	jobResult := result.(*model.ShowJobResponse)
	if jobResult.Entities == nil || jobResult.Entities.ImageId == nil {
		return "", fmt.Errorf("error extracting the image ID from API response")
	}

	imageID := *jobResult.Entities.ImageId
	return imageID, nil
}

func getImsJobStatus(client *ims.ImsClient, jobID string) ecsbuilder.StateRefreshFunc {
	return func() (interface{}, string, error) {
		jobRequest := &model.ShowJobRequest{
			JobId: jobID,
		}
		jobResponse, err := client.ShowJob(jobRequest)
		if err != nil {
			return nil, "", nil
		}

		jobStatus := jobResponse.Status.Value()

		if jobStatus == "FAIL" {
			return jobResponse, jobStatus, fmt.Errorf("failed to import image: %s", *jobResponse.FailReason)
		}
		return jobResponse, jobStatus, nil
	}
}

func (p *PostProcessor) newIMSClient(region string) (*ims.ImsClient, error) {
	hcClient, err := ecsbuilder.NewHcClient(&p.config.AccessConfig, region, "ims")
	if err != nil {
		return nil, err
	}

	return ims.NewImsClient(hcClient), nil
}
