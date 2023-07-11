package ecs

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
)

type stepAddImageMembers struct{}

func (s *stepAddImageMembers) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
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
	// the "image" maybe in format of "id1;id2;id3" when the image_type is data-disk or system-data
	sharedImages := strings.Split(imageId, ";")
	ui.Say(fmt.Sprintf("Adding members %v to image %s", config.ImageMembers, sharedImages))
	request := &model.BatchAddMembersRequest{
		Body: &model.BatchAddMembersRequestBody{
			Images:   sharedImages,
			Projects: config.ImageMembers,
		},
	}
	if _, err := imsClient.BatchAddMembers(request); err != nil {
		warnMessage := fmt.Sprintf("WARN: failed to add members to image: %s", err)
		ui.Message(warnMessage)
		ui.Message("WARN: please share the image manually!\n")
		return multistep.ActionContinue
	}

	if config.ImageAutoAcceptMembers {
		ui.Message("image_auto_accept_members is not supportted, please accept the image in the target project.")
	}

	return multistep.ActionContinue
}

func (s *stepAddImageMembers) Cleanup(multistep.StateBag) {
	// No cleanup...
}
