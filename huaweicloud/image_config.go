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
	// Glance metadata that will be applied to the image.
	ImageMetadata map[string]string `mapstructure:"metadata" required:"false"`
	// List of members to add to the image after creation. An image member is
	// usually a project (also called the "tenant") with whom the image is
	// shared.
	ImageMembers []string `mapstructure:"image_members" required:"false"`
	// List of tags to add to the image after creation.
	ImageTags []string `mapstructure:"image_tags" required:"false"`
	// Minimum disk size needed to boot image, in gigabytes.
	ImageMinDisk int `mapstructure:"image_min_disk" required:"false"`
}

func (c *ImageConfig) Prepare(ctx *interpolate.Context) []error {
	errs := make([]error, 0)
	if c.ImageName == "" {
		errs = append(errs, fmt.Errorf("image_name must be specified"))
	}

	// By default, OpenStack seems to create the image with an image_type of
	// "snapshot", since it came from snapshotting a VM. A "snapshot" looks
	// slightly different in the OpenStack UI and OpenStack won't show
	// "snapshot" images as a choice in the list of images to boot from for a
	// new instance. See https://github.com/hashicorp/packer/issues/3038
	if c.ImageMetadata == nil {
		c.ImageMetadata = map[string]string{"image_type": "image"}
	} else if c.ImageMetadata["image_type"] == "" {
		c.ImageMetadata["image_type"] = "image"
	}

	if c.ImageMinDisk < 0 {
		errs = append(errs, fmt.Errorf("An image min disk size must be greater than or equal to 0"))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
