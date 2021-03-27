package huaweicloud

import (
	"context"
	"fmt"

	"github.com/huaweicloud/golangsdk/openstack/imageservice/v2/members"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepAddImageMembers struct{}

func (s *stepAddImageMembers) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)

	imageId := state.Get("image").(string)

	if len(config.ImageMembers) == 0 {
		return multistep.ActionContinue
	}

	imageClient, err := config.imageV2Client()
	if err != nil {
		err = fmt.Errorf("Error initializing image service client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	for _, member := range config.ImageMembers {
		ui.Say(fmt.Sprintf("Adding member '%s' to image %s", member, imageId))
		r := members.Create(imageClient, imageId, member)
		if _, err = r.Extract(); err != nil {
			err = fmt.Errorf("Error adding member to image: %s", err)
			state.Put("error", err)
			return multistep.ActionHalt
		}
	}

	if config.ImageAutoAcceptMembers {
		projectId := imageClient.AKSKAuthOptions.ProjectId
		for _, member := range config.ImageMembers {
			ui.Say(fmt.Sprintf("Accepting image %s for member '%s'", imageId, member))
			imageClient.AKSKAuthOptions.ProjectId = member
			r := members.Update(imageClient, imageId, member, members.UpdateOpts{Status: "accepted"})
			if _, err = r.Extract(); err != nil {
				imageClient.AKSKAuthOptions.ProjectId = projectId
				err = fmt.Errorf("Error accepting image for member: %s", err)
				state.Put("error", err)
				return multistep.ActionHalt
			}
		}
		imageClient.AKSKAuthOptions.ProjectId = projectId
	}

	return multistep.ActionContinue
}

func (s *stepAddImageMembers) Cleanup(multistep.StateBag) {
	// No cleanup...
}
