package ecs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	ecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	ims "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
)

type stepCreateImage struct {
	WaitTimeout string
}

func (s *stepCreateImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	region := config.Region
	imsClient, err := config.HcImsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing image service client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Create the image
	ui.Say(fmt.Sprintf("Creating the %s image: %s ...", config.ImageType, config.ImageName))

	var waitTimeout time.Duration
	if s.WaitTimeout == "" {
		s.WaitTimeout = "30m"
	}

	waitTimeout, err = time.ParseDuration(s.WaitTimeout)
	if err != nil {
		log.Printf("[WARN] failed to parse `wait_image_ready_timeout` %s: %s", s.WaitTimeout, err)
		waitTimeout = 30 * time.Minute
	}

	var imageID string
	serverID := state.Get("server_id").(string)
	switch config.ImageType {
	case FullImageType:
		imageID, err = createServerWholeImage(ui, config, waitTimeout, imsClient, serverID)
	case DataImageType:
		imageID, err = createDataDiskImage(ui, config, waitTimeout, imsClient, serverID)
	default:
		imageID, err = CreateSystemImage(ui, config, waitTimeout, imsClient, serverID)
	}

	if err != nil {
		err := fmt.Errorf("Error creating image: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	ui.Message(fmt.Sprintf("Image: %s", imageID))
	state.Put("image", imageID)
	return multistep.ActionContinue
}

func (s *stepCreateImage) Cleanup(multistep.StateBag) {
	// No cleanup...
}

func buildImageTag(conf *Config) []model.TagKeyValue {
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

	return taglist
}

func CreateSystemImage(_ packer.Ui, conf *Config, timeout time.Duration, client *ims.ImsClient, serverID string) (string, error) {
	requestBody := model.CreateImageRequestBody{
		Name:        conf.ImageName,
		Description: &conf.ImageDescription,
		InstanceId:  &serverID,
	}

	if conf.EnterpriseProjectId != "" {
		requestBody.EnterpriseProjectId = &conf.EnterpriseProjectId
	}
	if taglist := buildImageTag(conf); taglist != nil {
		requestBody.ImageTags = &taglist
	}

	request := model.CreateImageRequest{
		Body: &requestBody,
	}

	log.Printf("[DEBUG] Create image options: %+v", requestBody)
	response, err := client.CreateImage(&request)
	if err != nil {
		return "", err
	}

	if response.JobId == nil {
		return "", fmt.Errorf("can not get the job from API response")
	}
	return waitImageJobSuccess(client, timeout, *response.JobId)
}

func createServerWholeImage(_ packer.Ui, conf *Config, timeout time.Duration, client *ims.ImsClient, serverID string) (string, error) {
	requestBody := model.CreateWholeImageRequestBody{
		Name:        conf.ImageName,
		Description: &conf.ImageDescription,
		InstanceId:  &serverID,
		VaultId:     &conf.Vault,
	}

	if conf.EnterpriseProjectId != "" {
		requestBody.EnterpriseProjectId = &conf.EnterpriseProjectId
	}
	if taglist := buildImageTag(conf); taglist != nil {
		requestBody.ImageTags = &taglist
	}

	request := model.CreateWholeImageRequest{
		Body: &requestBody,
	}

	log.Printf("[DEBUG] Create image options: %+v", requestBody)
	response, err := client.CreateWholeImage(&request)
	if err != nil {
		return "", err
	}

	if response.JobId == nil {
		return "", fmt.Errorf("can not get the job from API response")
	}
	return waitImageJobSuccess(client, timeout, *response.JobId)
}

type BlockDevice struct {
	VolumeId   string
	DeviceName string
}

func createDataDiskImage(ui packer.Ui, conf *Config, timeout time.Duration, client *ims.ImsClient, serverID string) (string, error) {
	region := conf.Region
	ecsClient, err := conf.HcEcsClient(region)
	if err != nil {
		return "", fmt.Errorf("Error initializing compute client: %s", err)
	}

	blockDevices, err := ecsClient.ListServerBlockDevices(&ecsmodel.ListServerBlockDevicesRequest{
		ServerId: serverID,
	})
	if err != nil {
		return "", err
	}

	if blockDevices == nil || blockDevices.VolumeAttachments == nil {
		return "", fmt.Errorf("failed to parse the response body")
	}

	volumes := make([]BlockDevice, 0, len(*blockDevices.VolumeAttachments))
	for _, dev := range *blockDevices.VolumeAttachments {
		if dev.BootIndex != nil && *dev.BootIndex == 0 {
			continue
		}

		diskNames := strings.Split(*dev.Device, "/")
		volumes = append(volumes, BlockDevice{
			VolumeId:   *dev.VolumeId,
			DeviceName: diskNames[len(diskNames)-1],
		})
	}

	if len(volumes) == 0 {
		return "", fmt.Errorf("no data disks attachmented to the ECS %s", serverID)
	}

	var allImages string
	for _, disk := range volumes {
		imageName := fmt.Sprintf("%s-%s", conf.ImageName, disk.DeviceName)
		dataImageOpts := []model.CreateDataImage{
			{
				Name:        imageName,
				VolumeId:    disk.VolumeId,
				Description: &conf.ImageDescription,
			},
		}
		requestBody := model.CreateImageRequestBody{
			DataImages: &dataImageOpts,
		}

		if conf.EnterpriseProjectId != "" {
			requestBody.EnterpriseProjectId = &conf.EnterpriseProjectId
		}
		if taglist := buildImageTag(conf); taglist != nil {
			requestBody.ImageTags = &taglist
		}

		request := model.CreateImageRequest{
			Body: &requestBody,
		}

		// create the data disk image one by one
		log.Printf("[DEBUG] Create data disk image for /dev/%s options: %+v", disk.DeviceName, requestBody)
		ui.Message(fmt.Sprintf("creating data disk image for /dev/%s ...", disk.DeviceName))
		response, err := client.CreateImage(&request)
		if err != nil {
			ui.Message(fmt.Sprintf("falid to create data disk image for /dev/%s: %s", disk.DeviceName, err))
			continue
		}

		if response.JobId == nil {
			ui.Message(fmt.Sprintf("can not get the job of creating data disk image for /dev/%s", disk.DeviceName))
			continue
		}

		imageID, err := waitImageJobSuccess(client, timeout, *response.JobId)
		if err != nil {
			ui.Message(fmt.Sprintf("Error waiting for data disk image /dev/%s: %s", disk.DeviceName, err))
			continue
		} else {
			ui.Message(fmt.Sprintf("data disk image for /dev/%s: %s", disk.DeviceName, imageID))
			allImages += ";" + imageID
		}
	}

	if allImages != "" {
		return allImages[1:], nil
	}
	return allImages, fmt.Errorf("all jobs are failed to create data disk image")
}

func waitImageJobSuccess(client *ims.ImsClient, timeout time.Duration, jobID string) (string, error) {
	stateConf := &StateChangeConf{
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
	imageID, err := getImageIDFromJobEntities(jobResult.Entities)
	if err != nil {
		return "", err
	}

	return imageID, nil
}

func getImsJobStatus(client *ims.ImsClient, jobID string) StateRefreshFunc {
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
			return jobResponse, jobStatus, fmt.Errorf("failed to create image: %s", *jobResponse.FailReason)
		}
		return jobResponse, jobStatus, nil
	}
}

func getImageIDFromJobEntities(entities *model.JobEntities) (string, error) {
	if entities == nil {
		return "", fmt.Errorf("Error extracting the image id from API response")
	}

	log.Printf("[DEBUG] the job Entities: %#v\n", entities)

	// for system image job and full-ECS iamge job
	if entities.ImageId != nil {
		return *entities.ImageId, nil
	}

	// for data disk image job
	if entities.SubJobsResult != nil && len(*entities.SubJobsResult) > 0 {
		subJobs := *entities.SubJobsResult
		subEntities := subJobs[0]

		if subEntities.Entities.ImageId != nil {
			return *subEntities.Entities.ImageId, nil
		}
	}

	return "", fmt.Errorf("Error extracting the image id from API response")
}
