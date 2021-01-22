package huaweicloud

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/huaweicloud/golangsdk/openstack/compute/v2/servers"
	"github.com/huaweicloud/golangsdk/openstack/ims/v2/cloudimages"
)

// timeoutCreate means the maximum time in seconds to create the image
const timeoutCreate int = 900

type stepCreateImage struct{}

func (s *stepCreateImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	server := state.Get("server").(*servers.Server)
	ui := state.Get("ui").(packer.Ui)

	// We need the v2 image client
	imageClient, err := config.imageV2Client()
	if err != nil {
		err = fmt.Errorf("Error initializing image service client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Create the image.
	ui.Say(fmt.Sprintf("Creating the image: %s", config.ImageName))

	taglist := []cloudimages.ImageTag{}
	for k, v := range config.ImageTags {
		tag := cloudimages.ImageTag{
			Key:   k,
			Value: v,
		}
		taglist = append(taglist, tag)
	}
	createOpts := &cloudimages.CreateByServerOpts{
		Name:        config.ImageName,
		Description: config.ImageDescription,
		InstanceId:  server.ID,
		ImageTags:   taglist,
	}
	log.Printf("[DEBUG] Create image options: %#v", createOpts)
	v, err := cloudimages.CreateImageByServer(imageClient, createOpts).ExtractJobResponse()
	if err != nil {
		err := fmt.Errorf("Error creating image: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	// Wait for the image to become available
	ui.Say(fmt.Sprintf("Waiting for image %s to become available ...", config.ImageName))
	err = cloudimages.WaitForJobSuccess(imageClient, timeoutCreate, v.JobID)
	if err != nil {
		err := fmt.Errorf("Error waiting for image: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	entity, err := cloudimages.GetJobEntity(imageClient, v.JobID, "image_id")
	if err != nil {
		err := fmt.Errorf("Error extracting image id: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	// Set the Image ID in the state
	imageID := entity.(string)
	ui.Message(fmt.Sprintf("Image: %s", imageID))
	state.Put("image", imageID)

	return multistep.ActionContinue
}

func (s *stepCreateImage) Cleanup(multistep.StateBag) {
	// No cleanup...
}
