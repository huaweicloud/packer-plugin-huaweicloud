//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type ImageFilter,ImageFilterOptions,DataVolume

package ecs

import (
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/hashicorp/packer-plugin-sdk/uuid"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
)

// RunConfig contains configuration for running an instance from a source image
// and details on how to access that launched image.
type RunConfig struct {
	Comm communicator.Config `mapstructure:",squash"`
	// The name for the desired flavor for the server to be created.
	Flavor string `mapstructure:"flavor" required:"true"`
	// The ID of Enterprise Project in which to create the image.
	// If omitted, the HW_ENTERPRISE_PROJECT_ID environment variable is used.
	EnterpriseProjectId string `mapstructure:"enterprise_project_id" required:"false"`
	// The availability zone to launch the server in.
	// If omitted, a random availability zone in the region will be used.
	AvailabilityZone string `mapstructure:"availability_zone" required:"false"`
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
	//             "name": "Ubuntu 20.04 server 64bit",
	//             "visibility": "public",
	//         },
	//         "most_recent": true
	//     }
	// }
	// ```
	//
	// This selects the most recent production Ubuntu 20.04 shared to you by
	// the given owner. NOTE: This will fail unless *exactly* one image is
	// returned, or `most_recent` is set to true. In the example of multiple
	// returned images, `most_recent` will cause this to succeed by selecting
	// the newest image of the returned images.
	//
	//   - `filters` (ImageFilterOptions) - filters used to select a `source_image`.
	//     NOTE: This will fail unless *exactly* one image is returned, or
	//     `most_recent` is set to true.
	//     The following filters are valid:
	//
	//     - name (string) - The image name. Exact matching is used.
	//     - owner (string) - The owner to which the image belongs.
	//     - visibility (string) - The visibility of the image. Available values include:
	//       *public*, *private*, *market*, and *shared*.
	//
	//   - `most_recent` (boolean) - Selects the newest created image when true.
	//     This is most useful for selecting a daily distro build.
	//
	// You may set use this in place of `source_image` if `source_image_filter`
	// is provided alongside `source_image`, the `source_image` will override
	// the filter. The filter will not be used in this case.
	SourceImageFilters ImageFilter `mapstructure:"source_image_filter" required:"false"`
	// A specific EIP ID to assign to this instance.
	FloatingIP string `mapstructure:"floating_ip" required:"false"`
	// Whether or not to attempt to reuse existing unassigned floating ips in
	// the project before allocating a new one. Note that it is not possible to
	// safely do this concurrently, so if you are running multiple builds
	// concurrently, or if other processes are assigning and using floating IPs
	// in the same project while packer is running, you should not set this to true.
	// Defaults to false.
	ReuseIPs bool `mapstructure:"reuse_ips" required:"false"`
	// The type of EIP. See the api doc to get the value.
	EIPType string `mapstructure:"eip_type" required:"false"`
	// The size of EIP bandwidth.
	EIPBandwidthSize int `mapstructure:"eip_bandwidth_size" required:"false"`
	// The IP version to use for SSH connections, valid values are `4` and `6`.
	SSHIPVersion string `mapstructure:"ssh_ip_version" required:"false"`
	// A vpc id to attach to this instance.
	VpcID string `mapstructure:"vpc_id" required:"false"`
	// A list of subnets by UUID to attach to this instance.
	Subnets []string `mapstructure:"subnets" required:"false"`
	// A list of security groups by name to add to this instance.
	SecurityGroups []string `mapstructure:"security_groups" required:"false"`
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
	// If set to true, the ECS will be billed in spot price mode.
	// This mode is more cost-effective than pay-per-use, and the spot price will be adjusted based on supply-and-demand changes.
	SpotPricing bool `mapstructure:"spot_pricing" required:"false"`
	// The highest price you are willing to pay for an ECS. This price is not lower than the current market price and
	// not higher than the pay-per-use price. When the market price is higher than your quoting or the inventory is insufficient,
	// the spot ECS will be terminated.
	SpotMaximumPrice string `mapstructure:"spot_maximum_price" required:"false"`
	// The system disk type of the instance. Defaults to `SSD`.
	// For details about disk types, see
	// [Disk Types and Disk Performance](https://support.huaweicloud.com/en-us/productdesc-evs/en-us_topic_0014580744.html).
	// Available values include:
	//   - `SAS`: high I/O disk type.
	//   - `SSD`: ultra-high I/O disk type.
	//   - `GPSSD`: general purpose SSD disk type.
	//   - `ESSD`: Extreme SSD type.
	VolumeType string `mapstructure:"volume_type" required:"false"`
	// The system disk size in GB. If this parameter is not specified,
	// it is set to the minimum value of the system disk in the source image.
	VolumeSize int `mapstructure:"volume_size" required:"false"`
	// Add one or more data disks to the instance before creating the image.
	// Only one of the four parameters of *volume_size*, *data_image_id*, *snapshot_id*, *volume_id*
	// can be selected at most.
	// If there is data disk that the value of *volume_id* or *snapshot_id* is not empty, the param of
	// *availability_zone* should be set and keep consistent with the data disk availability_zone.
	// If there are multiple data disks that the value of *volume_id* or *snapshot_id* is not empty, the data
	// disks should in the same availability_zone.
	// Usage example:
	//
	// ``` json {
	//   "data_disks": [
	//     {
	//       "volume_size": 100,
	//       "volume_type": "GPSSD"
	//     },
	//     {
	//       "data_image_id": "1cc1ccdd-7ef1-43b0-a8ea-c80ecf2f5da2",
	//       "volume_type": "GPSSD"
	//     },
	//     {
	//       "snapshot_id": "2f8a6e39-29d0-4fb9-9d7f-174e22ffa478",
	//       "volume_type": "GPSSD"
	//     },
	//     {
	//       "volume_id": "d2c9b3fd-7a72-4374-9502-00e8740bedbd",
	//       "volume_type": "GPSSD"
	//     }
	//   ],
	//   ...
	// }
	// ```
	//
	// The data_disks allow for the following argument:
	//   -  `volume_size` (int) - The data disk size in GB.
	//   -  `data_image_id` (string) - The ID of the data disk image.
	//   -  `snapshot_id` (string) - The ID of the snapshot.
	//   -  `volume_id` (string) - The ID of an existing volume.
	//   -  `volume_type` (string) - The data disk type of the instance. Defaults to `SSD`.
	//       Available values include: *SAS*, *SSD*, *GPSSD*, and *ESSD*.
	DataVolumes []DataVolume `mapstructure:"data_disks" required:"false"`
	// The ID of the vault to which the instance is to be added.
	// This parameter is **mandatory** when creating a full-ECS image from the instance.
	Vault string `mapstructure:"vault_id" required:"false"`

	sourceImageOpts *model.ListImagesRequest
}

type DataVolume struct {
	// The data disk size in GB.
	Size int `mapstructure:"volume_size" required:"false"`
	// The ID of the data disk image.
	DataImageId string `mapstructure:"data_image_id" required:"false"`
	// The ID of the snapshot.
	SnapshotId string `mapstructure:"snapshot_id" required:"false"`
	// The ID of an existing volume.
	VolumeId string `mapstructure:"volume_id" required:"false"`
	// The data disk type of the instance. Defaults to `SSD`.
	// Available values include: *SAS*, *SSD*, *GPSSD*, and *ESSD*.
	Type string `mapstructure:"volume_type" required:"false"`
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
	// Specifies the image name. Exact matching is used.
	Name string `mapstructure:"name"`
	// Specifies the owner to which the image belongs.
	Owner string `mapstructure:"owner"`
	// Whether the image is available to other tenants. Available values include:
	// *public*, *private*, *market*, and *shared*.
	Visibility string `mapstructure:"visibility"`
	// Specifies a tag added to an image. Tags can be used as a filter to query images.
	Tag string `mapstructure:"tag" required:"false"`
}

func (f *ImageFilterOptions) Empty() bool {
	return f.Name == "" && f.Owner == "" && f.Visibility == ""
}

func (f *ImageFilterOptions) Build() (*model.ListImagesRequest, error) {
	// Set defaults for status, sork_key, and sort_dir
	status := model.GetListImagesRequestStatusEnum().ACTIVE
	sortKey := model.GetListImagesRequestSortKeyEnum().CREATED_AT
	sortDir := model.GetListImagesRequestSortDirEnum().DESC
	opts := model.ListImagesRequest{
		Status:  &status,
		SortKey: &sortKey,
		SortDir: &sortDir,
	}

	if f.Name != "" {
		opts.Name = &f.Name
	}
	if f.Owner != "" {
		opts.Owner = &f.Owner
	}
	if f.Visibility != "" {
		v, err := getImageType(f.Visibility)
		if err != nil {
			return nil, err
		}
		opts.Imagetype = v
	}
	if f.Tag != "" {
		opts.Tag = &f.Tag
	}

	return &opts, nil
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

	if c.EnterpriseProjectId == "" {
		c.EnterpriseProjectId = os.Getenv("HW_ENTERPRISE_PROJECT_ID")
	}

	for key, value := range c.InstanceMetadata {
		if len(key) > 255 {
			errs = append(errs, fmt.Errorf("Instance metadata key too long (max 255 bytes): %s", key))
		}
		if len(value) > 255 {
			errs = append(errs, fmt.Errorf("Instance metadata value too long (max 255 bytes): %s", value))
		}
	}

	// if neither ID or image name is provided outside the filter, build the
	// filter
	if len(c.SourceImage) == 0 && len(c.SourceImageName) == 0 {
		listOpts, filterErr := c.SourceImageFilters.Filters.Build()
		if filterErr != nil {
			errs = append(errs, filterErr)
		}

		c.sourceImageOpts = listOpts
	}

	return errs
}

// Retrieve the specific ImageVisibility using the exported const from images
func getImageType(visibility string) (*model.ListImagesRequestImagetype, error) {
	var isValid bool
	supportedTypes := []string{"public", "private", "market", "shared"}
	for _, v := range supportedTypes {
		if visibility == v {
			isValid = true
			break
		}
	}
	if !isValid {
		return nil, fmt.Errorf("Not a valid visibility: expected to be one of %v, got %s", supportedTypes, visibility)
	}

	// actually, the *visibility* is used as *__imagetype*, we should convert public to gold
	if visibility == "public" {
		visibility = "gold"
	}

	var imageType model.ListImagesRequestImagetype
	err := imageType.UnmarshalJSON([]byte(visibility))
	if err != nil {
		return nil, fmt.Errorf("Error parsing the visibility %s: %s", visibility, err)
	}

	return &imageType, nil
}
