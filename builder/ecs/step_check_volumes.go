package ecs

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	evs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/evs/v2"
	evsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/evs/v2/model"
	ims "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2"
	imsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
)

type StepCheckVolumes struct {
	DataVolumes []DataVolume
}

func (s *StepCheckVolumes) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	if len(s.DataVolumes) == 0 {
		return multistep.ActionContinue
	}

	targetAZ := state.Get("availability_zone").(string)
	if err := checkDataVolumes(config, s.DataVolumes, targetAZ); err != nil {
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *StepCheckVolumes) Cleanup(state multistep.StateBag) {
}

func checkDataVolumes(config *Config, dataVolumes []DataVolume, targetAZ string) error {
	region := config.Region

	evsClient, err := config.HcEvsClient(region)
	if err != nil {
		return fmt.Errorf("error initializing EVS client: %s", err)
	}
	imsClient, err := config.HcImsClient(region)
	if err != nil {
		return fmt.Errorf("error initializing IMS client: %s", err)
	}

	// Accumulate any errors
	var errs *packer.MultiError
	allKeys := []string{"volume_size", "volume_id", "snapshot_id", "data_image_id"}
	for i, dataVolume := range dataVolumes {
		specified := make([]string, 0)
		if dataVolume.Size > 0 {
			specified = append(specified, "volume_size")
		}
		if dataVolume.VolumeId != "" {
			specified = append(specified, "volume_id")
		}
		if dataVolume.SnapshotId != "" {
			specified = append(specified, "snapshot_id")
		}
		if dataVolume.DataImageId != "" {
			specified = append(specified, "data_image_id")
		}

		if len(specified) == 0 {
			errs = packer.MultiErrorAppend(errs,
				fmt.Errorf("data_disks[%d]: one of `%s` must be specified", i, strings.Join(allKeys, ",")))
		}
		if len(specified) > 1 {
			errs = packer.MultiErrorAppend(errs,
				fmt.Errorf("data_disks[%d]: only one of `%s` can be specified, but `%s` were specified",
					i, strings.Join(allKeys, ","), strings.Join(specified, ",")))
		}
	}
	if errs != nil && len(errs.Errors) > 0 {
		return errs
	}

	for i, dataVolume := range dataVolumes {
		if dataVolume.VolumeId != "" {
			err = checkVolumeID(evsClient, dataVolume.VolumeId, targetAZ)
		}
		if dataVolume.SnapshotId != "" {
			err = checkSnapshotID(evsClient, dataVolume.SnapshotId, targetAZ)
		}
		if dataVolume.DataImageId != "" {
			err = checkDataImage(imsClient, dataVolume.DataImageId)
		}

		if err != nil {
			errs = packer.MultiErrorAppend(errs, fmt.Errorf("data_disks[%d]: %s", i, err))
		}
	}
	if errs != nil && len(errs.Errors) > 0 {
		return errs
	}

	return nil
}

func checkVolumeID(evsClient *evs.EvsClient, volumeID, targetAZ string) error {
	request := &evsmodel.ListVolumesRequest{
		Id:               &volumeID,
		AvailabilityZone: &targetAZ,
	}
	response, err := evsClient.ListVolumes(request)
	if err != nil {
		return err
	}

	if response.Volumes == nil || len(*response.Volumes) == 0 {
		return fmt.Errorf("can not find the volume %s in %s", volumeID, targetAZ)
	}
	return nil
}

func checkSnapshotID(evsClient *evs.EvsClient, snapshotID, targetAZ string) error {
	request := &evsmodel.ListSnapshotsRequest{
		Id:               &snapshotID,
		AvailabilityZone: &targetAZ,
	}
	response, err := evsClient.ListSnapshots(request)
	if err != nil {
		return err
	}

	if response.Snapshots == nil || len(*response.Snapshots) == 0 {
		return fmt.Errorf("can not find the snapshot %s in %s", snapshotID, targetAZ)
	}
	return nil
}

func checkDataImage(imsClient *ims.ImsClient, imageID string) error {
	request := &imsmodel.ListImagesRequest{
		Id: &imageID,
	}
	response, err := imsClient.ListImages(request)
	if err != nil {
		return err
	}

	if response.Images == nil || len(*response.Images) == 0 {
		return fmt.Errorf("can not find the image %s", imageID)
	}
	return nil
}
