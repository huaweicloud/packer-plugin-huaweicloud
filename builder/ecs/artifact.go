package ecs

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/packer"

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
	errors := make([]error, 0)
	log.Printf("Destroying image: %s", a.ImageId)

	for _, id := range strings.Split(a.ImageId, ";") {
		if id == "" {
			continue
		}

		request := model.GlanceDeleteImageRequest{
			ImageId: id,
		}
		if _, err := a.Client.GlanceDeleteImage(&request); err != nil {
			errors = append(errors, err)
			continue
		}
	}

	if len(errors) > 0 {
		if len(errors) == 1 {
			return errors[0]
		} else {
			return &packer.MultiError{Errors: errors}
		}
	}

	return nil
}
