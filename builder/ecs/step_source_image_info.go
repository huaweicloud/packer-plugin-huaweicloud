package ecs

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
)

type StepSourceImageInfo struct {
	SourceImage      string
	SourceImageName  string
	SourceMostRecent bool
	SourceImageOpts  *model.ListImagesRequest
}

func (s *StepSourceImageInfo) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)

	if s.SourceImage != "" {
		state.Put("source_image", s.SourceImage)
		return multistep.ActionContinue
	}

	region := config.Region
	client, err := config.HcImsClient(region)
	if err != nil {
		err := fmt.Errorf("error creating image client: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	// update the image name if necessary
	if s.SourceImageName != "" {
		if s.SourceImageOpts == nil {
			s.SourceImageOpts = &model.ListImagesRequest{
				Name: &s.SourceImageName,
			}
		} else {
			s.SourceImageOpts.Name = &s.SourceImageName
		}
	}

	log.Printf("Using Image Filters %+v", *s.SourceImageOpts)
	response, err := client.ListImages(s.SourceImageOpts)
	if err != nil {
		err := fmt.Errorf("Error querying image: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	images := *response.Images
	if len(images) == 0 {
		err := fmt.Errorf("No image was found matching filters: %+v",
			*s.SourceImageOpts)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	if len(images) > 1 && !s.SourceMostRecent {
		err := fmt.Errorf("Your query returned more than one result. Please try a more specific search, or set most_recent to true. Search filters: %+v",
			*s.SourceImageOpts)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt

	}

	imageID := images[0].Id
	ui.Message(fmt.Sprintf("Found Image ID: %s", imageID))

	state.Put("source_image", imageID)
	return multistep.ActionContinue
}

func (s *StepSourceImageInfo) Cleanup(state multistep.StateBag) {
	// No cleanup required for backout
}
