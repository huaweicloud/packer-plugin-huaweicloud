package ecs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/huaweicloud/golangsdk/openstack/compute/v2/servers"

	ims "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
)

type stepCreateImage struct{}

func (s *stepCreateImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	server := state.Get("server").(*servers.Server)
	ui := state.Get("ui").(packer.Ui)

	region := config.Region
	imsClient, err := config.HcImsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing image service client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Create the image
	ui.Say(fmt.Sprintf("Creating the image: %s", config.ImageName))

	taglist := make([]model.TagKeyValue, len(config.ImageTags))
	index := 0
	for k, v := range config.ImageTags {
		taglist[index] = model.TagKeyValue{
			Key:   k,
			Value: v,
		}
		index++
	}

	requestBody := model.CreateImageRequestBody{
		Name:        config.ImageName,
		Description: &config.ImageDescription,
		InstanceId:  &server.ID,
		ImageTags:   &taglist,
	}
	request := model.CreateImageRequest{
		Body: &requestBody,
	}

	log.Printf("[DEBUG] Create image options: %+v", requestBody)
	response, err := imsClient.CreateImage(&request)
	if err != nil {
		err := fmt.Errorf("Error creating image: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	// Wait for the image to become available
	ui.Say(fmt.Sprintf("Waiting for image %s to become available ...", config.ImageName))
	jobID := *response.JobId
	stateConf := &StateChangeConf{
		Pending:      []string{"INIT", "RUNNING"},
		Target:       []string{"SUCCESS"},
		Refresh:      getImsJobStatus(imsClient, jobID),
		Timeout:      10 * time.Minute,
		Delay:        10 * time.Second,
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
		return jobResponse, jobStatus, nil
	}
}
