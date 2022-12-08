package ecs

import (
	"fmt"
	"log"

	ims "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
)

// Artifact is an artifact implementation that contains built images.
type Artifact struct {
	// ImageId of built image
	ImageId string

	// BuilderIdValue is the unique ID for the builder that created this image
	BuilderIdValue string

	// IMS client for performing API stuff.
	Client *ims.ImsClient
}

func (a *Artifact) BuilderId() string {
	return a.BuilderIdValue
}

func (*Artifact) Files() []string {
	// We have no files
	return nil
}

func (a *Artifact) Id() string {
	return a.ImageId
}

func (a *Artifact) String() string {
	return fmt.Sprintf("An image was created: %v", a.ImageId)
}

func (a *Artifact) State(name string) interface{} {
	return nil
}

func (a *Artifact) Destroy() error {
	log.Printf("Destroying image: %s", a.ImageId)

	request := model.GlanceDeleteImageRequest{
		ImageId: a.ImageId,
	}
	_, err := a.Client.GlanceDeleteImage(&request)
	return err
}
