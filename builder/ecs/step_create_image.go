package ecs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

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
	ui.Say(fmt.Sprintf("Creating the image: %s", config.ImageName))

	var jobID string
	var createErr error

	serverID := state.Get("server_id").(string)
	if len(config.DataVolumes) == 0 {
		jobID, createErr = createServerImage(config, imsClient, serverID)
	} else {
		jobID, createErr = createServerWholeImage(config, imsClient, serverID)
	}

	if createErr != nil {
		err := fmt.Errorf("Error creating image: %s", createErr)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	var waitTimeout time.Duration
	if s.WaitTimeout == "" {
		s.WaitTimeout = "30m"
	}

	waitTimeout, err = time.ParseDuration(s.WaitTimeout)
	if err != nil {
		log.Printf("[WARN] failed to parse `wait_image_ready_timeout` %s: %s", s.WaitTimeout, err)
		waitTimeout = 30 * time.Minute
	}

	// Wait for the image to become available
	ui.Say(fmt.Sprintf("Waiting for image %s to become available in %s ...", config.ImageName, waitTimeout))
	stateConf := &StateChangeConf{
		Pending:      []string{"INIT", "RUNNING"},
		Target:       []string{"SUCCESS"},
		Refresh:      getImsJobStatus(imsClient, jobID),
		Timeout:      waitTimeout,
		Delay:        60 * time.Second,
		PollInterval: 10 * time.Second,
		StateBag:     state,
	}

	result, err := stateConf.WaitForState()
	if err != nil {
		err := fmt.Errorf("Error waiting for image: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	jobResult := result.(*model.ShowJobResponse)
	if jobResult.Entities == nil {
		err := fmt.Errorf("Error extracting image id: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	imageID := *jobResult.Entities.ImageId
	ui.Message(fmt.Sprintf("Image: %s", imageID))
	state.Put("image", imageID)

	return multistep.ActionContinue
}

func (s *stepCreateImage) Cleanup(multistep.StateBag) {
	// No cleanup...
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

func createServerImage(conf *Config, client *ims.ImsClient, serverID string) (string, error) {
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
	return *response.JobId, nil
}

func createServerWholeImage(conf *Config, client *ims.ImsClient, serverID string) (string, error) {
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
	return *response.JobId, nil
}
