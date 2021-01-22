//go:generate struct-markdown

package huaweicloud

import (
	"fmt"

	"github.com/hashicorp/packer/template/interpolate"
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
	// Minimum disk size needed to boot image, in gigabytes.
	ImageMinDisk int `mapstructure:"image_min_disk" required:"false"`
	// The tags of the image in key/pair format.
	ImageTags map[string]string `mapstructure:"image_tags" required:"false"`
}

func (c *ImageConfig) Prepare(ctx *interpolate.Context) []error {
	errs := make([]error, 0)
	if c.ImageName == "" {
		errs = append(errs, fmt.Errorf("image_name must be specified"))
	}

	if c.ImageMinDisk < 0 {
		errs = append(errs, fmt.Errorf("An image min disk size must be greater than or equal to 0"))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
