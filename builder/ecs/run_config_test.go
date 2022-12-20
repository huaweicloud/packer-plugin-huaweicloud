package ecs

import (
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/mitchellh/mapstructure"
)

func testRunConfig() *RunConfig {
	return &RunConfig{
		SourceImage: "abcd",
		Flavor:      "m1.small",

		Comm: communicator.Config{
			SSH: communicator.SSH{
				SSHUsername: "foo",
			},
		},
	}
}

func TestRunConfigPrepare(t *testing.T) {
	c := testRunConfig()
	err := c.Prepare(nil)
	if len(err) > 0 {
		t.Fatalf("err: %s", err)
	}
}

func TestRunConfigPrepare_InstanceType(t *testing.T) {
	c := testRunConfig()
	c.Flavor = ""
	if err := c.Prepare(nil); len(err) != 1 {
		t.Fatalf("err: %s", err)
	}
}

func TestRunConfigPrepare_SourceImage(t *testing.T) {
	c := testRunConfig()
	c.SourceImage = ""
	if err := c.Prepare(nil); len(err) != 1 {
		t.Fatalf("err: %s", err)
	}
}

func TestRunConfigPrepare_SSHPort(t *testing.T) {
	c := testRunConfig()
	c.Comm.SSHPort = 0
	if err := c.Prepare(nil); len(err) != 0 {
		t.Fatalf("err: %s", err)
	}

	if c.Comm.SSHPort != 22 {
		t.Fatalf("invalid value: %d", c.Comm.SSHPort)
	}

	c.Comm.SSHPort = 44
	if err := c.Prepare(nil); len(err) != 0 {
		t.Fatalf("err: %s", err)
	}

	if c.Comm.SSHPort != 44 {
		t.Fatalf("invalid value: %d", c.Comm.SSHPort)
	}
}

func TestRunConfigPrepare_BlockStorage(t *testing.T) {
	c := testRunConfig()
	c.VolumeType = "fast"
	c.AvailabilityZone = "RegionOne"

	if err := c.Prepare(nil); len(err) != 0 {
		t.Fatalf("err: %s", err)
	}

	if c.VolumeType != "fast" {
		t.Fatalf("invalid value: %s", c.VolumeType)
	}
	if c.AvailabilityZone != "RegionOne" {
		t.Fatalf("invalid value: %s", c.AvailabilityZone)
	}
}

// This test case confirms that only allowed fields will be set to values
// The checked values are non-nil for their target type
func TestBuildImageFilter(t *testing.T) {

	filters := ImageFilterOptions{
		Name:       "Ubuntu 20.04",
		Visibility: "public",
		Owner:      "1234567890",
	}

	listOpts, err := filters.Build()
	if err != nil {
		t.Errorf("Building filter failed with: %s", err)
	}

	if *listOpts.Name != "Ubuntu 20.04" {
		t.Errorf("Name did not build correctly: %s", *listOpts.Name)
	}

	if *listOpts.Owner != "1234567890" {
		t.Errorf("Owner did not build correctly: %s", *listOpts.Owner)
	}

	imageType := listOpts.Imagetype.Value()
	if imageType == "gold" {
		imageType = "public"
	}
	if imageType != "public" {
		t.Errorf("Visibility did not build correctly: %s", imageType)
	}
}

func TestBuildBadImageFilter(t *testing.T) {
	filterMap := map[string]interface{}{
		"limit":   "100",
		"min_ram": "1024",
	}

	filters := ImageFilterOptions{}
	mapstructure.Decode(filterMap, &filters)
	listOpts, err := filters.Build()

	if err != nil {
		t.Errorf("Error returned processing image filter: %s", err.Error())
		return // we cannot trust listOpts to not cause unexpected behaviour
	}

	if listOpts.Limit != nil && *listOpts.Limit == filterMap["limit"] {
		t.Errorf("Limit was parsed into ListOpts: %d", listOpts.Limit)
	}

	if listOpts.MinRam != nil && *listOpts.MinRam != 0 {
		t.Errorf("MinRam was parsed into ListOpts: %d", *listOpts.MinRam)
	}

	if !filters.Empty() {
		t.Errorf("The filters should be empty due to lack of input")
	}
}

// Tests that the Empty method on ImageFilterOptions works as expected
func TestImageFiltersEmpty(t *testing.T) {
	filledFilters := ImageFilterOptions{
		Name:       "Ubuntu 20.04",
		Visibility: "public",
		Owner:      "1234567890",
	}

	if filledFilters.Empty() {
		t.Errorf("Expected filled filters to be non-empty: %v", filledFilters)
	}

	emptyFilters := ImageFilterOptions{}

	if !emptyFilters.Empty() {
		t.Errorf("Expected default filter to be empty: %v", emptyFilters)
	}
}
