package ecs

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
)

type stepAddImageMembers struct{}

func (s *stepAddImageMembers) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)

	if len(config.ImageMembers) == 0 {
		return multistep.ActionContinue
	}

	region := config.Region
	imsClient, err := config.HcImsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing image service client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	imageId := state.Get("image").(string)
	ui.Say(fmt.Sprintf("Adding members %v to image %s", config.ImageMembers, imageId))
	request := &model.BatchAddMembersRequest{
		Body: &model.BatchAddMembersRequestBody{
			Images:   []string{imageId},
			Projects: config.ImageMembers,
		},
	}
	if _, err := imsClient.BatchAddMembers(request); err != nil {
		err = fmt.Errorf("Error adding member to image: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	if config.ImageAutoAcceptMembers {
		ui.Message("image_auto_accept_members is not supportted, please accept the image in the target project.")
	}

	return multistep.ActionContinue
}

func (s *stepAddImageMembers) Cleanup(multistep.StateBag) {
	// No cleanup...
}
