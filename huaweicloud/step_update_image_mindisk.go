package huaweicloud

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
	"github.com/huaweicloud/golangsdk/openstack/imageservice/v2/images"
)

type stepUpdateImageMinDisk struct{}

func (s *stepUpdateImageMinDisk) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	imageId := state.Get("image").(string)
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	if config.ImageMinDisk == 0 {
		return multistep.ActionContinue
	}
	imageClient, err := config.imageV2Client()
	if err != nil {
		err := fmt.Errorf("Error initializing image service client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Updating image min disk to %d", config.ImageMinDisk))

	r := images.Update(
		imageClient,
		imageId,
		images.UpdateOpts{
			images.UpdateImageProperty{
				Op:    "replace",
				Name:  "/min_disk",
				Value: string(config.ImageMinDisk),
			},
		},
	)

	if _, err := r.Extract(); err != nil {
		err = fmt.Errorf("Error updating image min disk: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *stepUpdateImageMinDisk) Cleanup(multistep.StateBag) {
	// No cleanup...
}
