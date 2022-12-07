//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type ImageFilter,ImageFilterOptions

package ecs

import (
	"errors"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/hashicorp/packer-plugin-sdk/uuid"
	"github.com/huaweicloud/golangsdk/openstack/imageservice/v2/images"
)

// RunConfig contains configuration for running an instance from a source image
// and details on how to access that launched image.
type RunConfig struct {
	Comm communicator.Config `mapstructure:",squash"`
	// The type of interface to connect via SSH, valid values are "public"
	// and "private", and the default behavior is to connect via
	// whichever is returned first from the HuaweiCloud API.
	SSHInterface string `mapstructure:"ssh_interface" required:"false"`
	// The IP version to use for SSH connections, valid values are `4` and `6`.
	// Useful on dual stacked instances where the default behavior is to
	// connect via whichever IP address is returned first from the HuaweiCloud
	// API.
	SSHIPVersion string `mapstructure:"ssh_ip_version" required:"false"`
	// The ID of the base image to use. This is the image that will
	// be used to launch a new server and provision it. Unless you specify
	// completely custom SSH settings, the source image must have cloud-init
	// installed so that the keypair gets assigned properly.
	SourceImage string `mapstructure:"source_image" required:"false"`
	// The name of the base image to use. This is an alternative way of
	// providing source_image and only either of them can be specified.
	SourceImageName string `mapstructure:"source_image_name" required:"false"`
	// Filters used to populate filter options. Example:
	//
	// ``` json {
	//     "source_image_filter": {
	//         "filters": {
	//             "name": "Ubuntu 18.04 server 64bit",
	//             "visibility": "public",
	//         },
	//         "most_recent": true
	//     }
	// }
	// ```
	//
	// This selects the most recent production Ubuntu 16.04 shared to you by
	// the given owner. NOTE: This will fail unless *exactly* one image is
	// returned, or `most_recent` is set to true. In the example of multiple
	// returned images, `most_recent` will cause this to succeed by selecting
	// the newest image of the returned images.
	//
	// -   `filters` (map of strings) - filters used to select a
	// `source_image`.
	//     NOTE: This will fail unless *exactly* one image is returned, or
	//     `most_recent` is set to true.
	//     The following filters are valid:
	//
	//     -   name (string)
	//     -   owner (string)
	//     -   visibility (string)
	//     -   properties (map of strings to strings)
	//
	// -   `most_recent` (boolean) - Selects the newest created image when
	// true.
	//     This is most useful for selecting a daily distro build.
	//
	// You may set use this in place of `source_image` If `source_image_filter`
	// is provided alongside `source_image`, the `source_image` will override
	// the filter. The filter will not be used in this case.
	SourceImageFilters ImageFilter `mapstructure:"source_image_filter" required:"false"`
	// The ID or name for the desired flavor for the server to be created.
	Flavor string `mapstructure:"flavor" required:"true"`
	// The availability zone to launch the server in.
	// If omitted, a random availability zone in the region will be used.
	AvailabilityZone string `mapstructure:"availability_zone" required:"false"`
	// A specific floating IP to assign to this instance.
	FloatingIP string `mapstructure:"floating_ip" required:"false"`
	// Whether or not to attempt to reuse existing unassigned floating ips in
	// the project before allocating a new one. Note that it is not possible to
	// safely do this concurrently, so if you are running multiple builds
	// concurrently, or if other processes are assigning and using floating IPs
	// in the same project while packer is running, you should not set this to true.
	// Defaults to false.
	ReuseIPs bool `mapstructure:"reuse_ips" required:"false"`
	// The type of eip. See the api doc to get the value.
	EIPType string `mapstructure:"eip_type" required:"false"`
	// The size of eip bandwidth.
	EIPBandwidthSize int `mapstructure:"eip_bandwidth_size" required:"false"`
	// A list of security groups by name to add to this instance.
	SecurityGroups []string `mapstructure:"security_groups" required:"false"`
	// A list of networks by UUID to attach to this instance.
	Networks []string `mapstructure:"networks" required:"false"`
	// A vpc id to attach to this instance.
	VpcID string `mapstructure:"vpc_id" required:"false"`
	// A list of subnets by UUID to attach to this instance.
	Subnets []string `mapstructure:"subnets" required:"false"`
	// User data to apply when launching the instance. Note that you need to be
	// careful about escaping characters due to the templates being JSON. It is
	// often more convenient to use user_data_file, instead. Packer will not
	// automatically wait for a user script to finish before shutting down the
	// instance this must be handled in a provisioner.
	UserData string `mapstructure:"user_data" required:"false"`
	// Path to a file that will be used for the user data when launching the
	// instance.
	UserDataFile string `mapstructure:"user_data_file" required:"false"`
	// Name that is applied to the server instance created by Packer. If this
	// isn't specified, the default is same as image_name.
	InstanceName string `mapstructure:"instance_name" required:"false"`
	// Metadata that is applied to the server instance created by Packer. Also
	// called server properties in some documentation. The strings have a max
	// size of 255 bytes each.
	InstanceMetadata map[string]string `mapstructure:"instance_metadata" required:"false"`
	// Whether or not nova should use ConfigDrive for cloud-init metadata.
	ConfigDrive bool `mapstructure:"config_drive" required:"false"`
	// Name of the Block Storage service volume. If this isn't specified,
	// random string will be used.
	VolumeName string `mapstructure:"volume_name" required:"false"`
	// Type of the Block Storage service volume.
	VolumeType string `mapstructure:"volume_type" required:"false"`
	// Size of the Block Storage service volume in GB. If this isn't specified,
	// it is set to source image min disk value (if set) or calculated from the
	// source image bytes size. Note that in some cases this needs to be
	// specified, if use_blockstorage_volume is true.
	VolumeSize int `mapstructure:"volume_size" required:"false"`

	sourceImageOpts images.ListOpts
}

type ImageFilter struct {
	// filters used to select a source_image. NOTE: This will fail unless
	// exactly one image is returned, or most_recent is set to true.
	Filters ImageFilterOptions `mapstructure:"filters" required:"false"`
	// Selects the newest created image when true. This is most useful for
	// selecting a daily distro build.
	MostRecent bool `mapstructure:"most_recent" required:"false"`
}

type ImageFilterOptions struct {
	Name       string            `mapstructure:"name"`
	Owner      string            `mapstructure:"owner"`
	Visibility string            `mapstructure:"visibility"`
	Properties map[string]string `mapstructure:"properties"`
}

func (f *ImageFilterOptions) Empty() bool {
	return f.Name == "" && f.Owner == "" && f.Visibility == "" && len(f.Properties) == 0
}

func (f *ImageFilterOptions) Build() (*images.ListOpts, error) {
	opts := images.ListOpts{}
	// Set defaults for status, member_status, and sort
	opts.Status = images.ImageStatusActive
	opts.MemberStatus = images.ImageMemberStatusAccepted
	opts.Sort = "created_at:desc"

	var err error

	if f.Name != "" {
		opts.Name = f.Name
	}
	if f.Owner != "" {
		opts.Owner = f.Owner
	}
	if f.Visibility != "" {
		v, err := getImageVisibility(f.Visibility)
		if err == nil {
			opts.Visibility = *v
		}
	}

	return &opts, err
}

func (c *RunConfig) Prepare(ctx *interpolate.Context) []error {
	// If we are not given an explicit ssh_keypair_name or
	// ssh_private_key_file, then create a temporary one, but only if the
	// temporary_key_pair_name has not been provided and we are not using
	// ssh_password.
	if c.Comm.SSHKeyPairName == "" && c.Comm.SSHTemporaryKeyPairName == "" &&
		c.Comm.SSHPrivateKeyFile == "" && c.Comm.SSHPassword == "" {

		c.Comm.SSHTemporaryKeyPairName = fmt.Sprintf("packer_%s", uuid.TimeOrderedUUID())
	}

	// Validation
	errs := c.Comm.Prepare(ctx)

	if c.Comm.SSHKeyPairName != "" {
		if c.Comm.Type == "winrm" && c.Comm.WinRMPassword == "" && c.Comm.SSHPrivateKeyFile == "" {
			errs = append(errs, errors.New("A ssh_private_key_file must be provided to retrieve the winrm password when using ssh_keypair_name."))
		} else if c.Comm.SSHPrivateKeyFile == "" && !c.Comm.SSHAgentAuth {
			errs = append(errs, errors.New("A ssh_private_key_file must be provided or ssh_agent_auth enabled when ssh_keypair_name is specified."))
		}
	}

	if c.SourceImage == "" && c.SourceImageName == "" && c.SourceImageFilters.Filters.Empty() {
		errs = append(errs, errors.New("Either a source_image, a source_image_name, or source_image_filter must be specified"))
	} else if len(c.SourceImage) > 0 && len(c.SourceImageName) > 0 {
		errs = append(errs, errors.New("Only a source_image or a source_image_name can be specified, not both."))
	}

	if c.Flavor == "" {
		errs = append(errs, errors.New("A flavor must be specified"))
	}

	if c.SSHIPVersion != "" && c.SSHIPVersion != "4" && c.SSHIPVersion != "6" {
		errs = append(errs, errors.New("SSH IP version must be either 4 or 6"))
	}

	for key, value := range c.InstanceMetadata {
		if len(key) > 255 {
			errs = append(errs, fmt.Errorf("Instance metadata key too long (max 255 bytes): %s", key))
		}
		if len(value) > 255 {
			errs = append(errs, fmt.Errorf("Instance metadata value too long (max 255 bytes): %s", value))
		}
	}

	// Use random name for the Block Storage volume if it's not provided.
	if c.VolumeName == "" {
		c.VolumeName = fmt.Sprintf("packer_%s", uuid.TimeOrderedUUID())
	}

	// if neither ID or image name is provided outside the filter, build the
	// filter
	if len(c.SourceImage) == 0 && len(c.SourceImageName) == 0 {

		listOpts, filterErr := c.SourceImageFilters.Filters.Build()

		if filterErr != nil {
			errs = append(errs, filterErr)
		}
		c.sourceImageOpts = *listOpts
	}

	return errs
}

// Retrieve the specific ImageVisibility using the exported const from images
func getImageVisibility(visibility string) (*images.ImageVisibility, error) {
	visibilities := [...]images.ImageVisibility{
		images.ImageVisibilityPublic,
		images.ImageVisibilityPrivate,
		images.ImageVisibilityCommunity,
		images.ImageVisibilityShared,
	}

	for _, v := range visibilities {
		if string(v) == visibility {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("Not a valid visibility: %s", visibility)
}
