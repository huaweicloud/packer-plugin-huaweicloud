//go:generate packer-sdc struct-markdown

package ecs

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

// ImageConfig is for common configuration related to creating Images.
type ImageConfig struct {
	// The name of the packer image.
	ImageName string `mapstructure:"image_name" required:"true"`
	// The description of the packer image.
	ImageDescription string `mapstructure:"image_description" required:"false"`
	// The type of the packer image. Available values include:
	// *system*, *data-disk*, *system-data* and *full-ecs*.
	ImageType string `mapstructure:"image_type" required:"false"`
	// The tags of the packer image in key/value format.
	ImageTags map[string]string `mapstructure:"image_tags" required:"false"`
	// List of members to add to the image after creation. An image member is
	// usually a project (also called the "tenant") with whom the image is
	// shared.
	ImageMembers []string `mapstructure:"image_members" required:"false"`
	// **Deprecated**. When true, perform the image accept so the members can see the image in their
	// project. This requires a user with priveleges both in the build project and
	// in the members provided. Defaults to false.
	ImageAutoAcceptMembers bool `mapstructure:"image_auto_accept_members" required:"false"`
	// Timeout of creating the image. The timeout string is a possibly signed sequence of
	// decimal numbers, each with optional fraction and a unit suffix, such as "40m", "1.5h" or "2h30m".
	// The default timeout is "30m" which means 30 minutes.
	WaitImageReadyTimeout string `mapstructure:"wait_image_ready_timeout" required:"false"`
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
