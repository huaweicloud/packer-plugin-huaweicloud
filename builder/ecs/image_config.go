//go:generate packer-sdc struct-markdown

package ecs

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

// ImageConfig is for common configuration related to creating Images.
type ImageConfig struct {
	// The name of the resulting image.
	ImageName string `mapstructure:"image_name" required:"true"`
	// Specifies the image description.
	ImageDescription string `mapstructure:"image_description" required:"false"`
	// List of members to add to the image after creation. An image member is
	// usually a project (also called the "tenant") with whom the image is
	// shared.
	ImageMembers []string `mapstructure:"image_members" required:"false"`
	// When true, perform the image accept so the members can see the image in their
	// project. This requires a user with priveleges both in the build project and
	// in the members provided. Defaults to false.
	ImageAutoAcceptMembers bool `mapstructure:"image_auto_accept_members" required:"false"`
	// The tags of the image in key/pair format.
	ImageTags map[string]string `mapstructure:"image_tags" required:"false"`
}

func (c *ImageConfig) Prepare(ctx *interpolate.Context) []error {
	errs := make([]error, 0)
	if c.ImageName == "" {
		errs = append(errs, fmt.Errorf("image_name must be specified"))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
