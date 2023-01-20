package ecs

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	evs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/evs/v2"
	evsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/evs/v2/model"
	ims "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2"
	imsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2/model"
)

type DataVolumeType int

const (
	VolumeId DataVolumeType = iota
	Size
	DataImageId
	SnapshotId
)

type StepCheckVolumes struct {
	DataVolumes []DataVolume
}

type DataVolumeWrap struct {
	DataVolume
	dataType   DataVolumeType
	volumeName string
	serverId   string
}

func (s *StepCheckVolumes) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	if len(s.DataVolumes) == 0 {
		return multistep.ActionContinue
	}

	targetAZ := state.Get("availability_zone").(string)
	dataVolumeWraps, err := checkAndWrapDataVolumes(config, s.DataVolumes, targetAZ)
	if err != nil {
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	state.Put("data_disk_wraps", dataVolumeWraps)
	return multistep.ActionContinue
}

func (s *StepCheckVolumes) Cleanup(state multistep.StateBag) {
}

func checkAndWrapDataVolumes(config *Config, dataVolumes []DataVolume, targetAZ string) ([]DataVolumeWrap, error) {
	region := config.Region
	evsClient, err := config.HcEvsClient(region)
	if err != nil {
		return nil, fmt.Errorf("error initializing EVS client: %s", err)
	}
	imsClient, err := config.HcImsClient(region)
	if err != nil {
		return nil, fmt.Errorf("error initializing IMS client: %s", err)
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
	// return error if the format is invalid
	if errs != nil && len(errs.Errors) > 0 {
		return nil, errs
	}

	var dataVolumeWraps = make([]DataVolumeWrap, 0, len(dataVolumes))
	for i := range dataVolumes {
		dataVolume := dataVolumes[i]

		if dataVolume.Size > 0 {
			dataVolumeWraps = append(dataVolumeWraps, DataVolumeWrap{
				DataVolume: dataVolume,
				dataType:   Size,
			})
		}

		if dataVolume.VolumeId != "" {
			if volumeSize, err := checkVolumeID(evsClient, dataVolume.VolumeId, targetAZ); err == nil {
				dataVolume.Size = volumeSize
				dataVolumeWraps = append(dataVolumeWraps, DataVolumeWrap{
					DataVolume: dataVolume,
					dataType:   VolumeId,
				})
			} else {
				errs = packer.MultiErrorAppend(errs, fmt.Errorf("data_disks[%d]: %s", i, err))
			}
		}

		if dataVolume.SnapshotId != "" {
			if volumeSize, err := checkSnapshotID(evsClient, dataVolume.SnapshotId, targetAZ); err == nil {
				dataVolume.Size = volumeSize
				dataVolumeWraps = append(dataVolumeWraps, DataVolumeWrap{
					DataVolume: dataVolume,
					dataType:   SnapshotId,
				})
			} else {
				errs = packer.MultiErrorAppend(errs, fmt.Errorf("data_disks[%d]: %s", i, err))
			}
		}

		if dataVolume.DataImageId != "" {
			if volumeSize, err := checkDataImage(imsClient, dataVolume.DataImageId); err == nil {
				dataVolume.Size = volumeSize
				dataVolumeWraps = append(dataVolumeWraps, DataVolumeWrap{
					DataVolume: dataVolume,
					dataType:   DataImageId,
				})
			} else {
				errs = packer.MultiErrorAppend(errs, fmt.Errorf("data_disks[%d]: %s", i, err))
			}
		}
	}
	if errs != nil && len(errs.Errors) > 0 {
		return nil, errs
	}

	return dataVolumeWraps, nil
}

func checkVolumeID(evsClient *evs.EvsClient, volumeID, targetAZ string) (int, error) {
	var volumeSize int

	request := &evsmodel.ListVolumesRequest{
		Id:               &volumeID,
		AvailabilityZone: &targetAZ,
	}
	response, err := evsClient.ListVolumes(request)
	if err != nil {
		return volumeSize, err
	}

	if response.Volumes == nil || len(*response.Volumes) == 0 {
		return volumeSize, fmt.Errorf("can not find the volume %s in %s", volumeID, targetAZ)
	}

	all := *response.Volumes
	volumeSize = int(all[0].Size)
	log.Printf("[DEBUG] the volume size of %s is %d GB", volumeID, volumeSize)
	return volumeSize, nil
}

func checkSnapshotID(evsClient *evs.EvsClient, snapshotID, targetAZ string) (int, error) {
	var volumeSize int

	request := &evsmodel.ListSnapshotsRequest{
		Id:               &snapshotID,
		AvailabilityZone: &targetAZ,
	}
	response, err := evsClient.ListSnapshots(request)
	if err != nil {
		return volumeSize, err
	}

	if response.Snapshots == nil || len(*response.Snapshots) == 0 {
		return volumeSize, fmt.Errorf("can not find the snapshot %s in %s", snapshotID, targetAZ)
	}

	all := *response.Snapshots
	volumeSize = int(all[0].Size)
	log.Printf("[DEBUG] the target volume size of snapshot %s is %d GB", snapshotID, volumeSize)
	return volumeSize, nil
}

func checkDataImage(imsClient *ims.ImsClient, imageID string) (int, error) {
	var volumeSize int

	request := &imsmodel.ListImagesRequest{
		Id: &imageID,
	}
	response, err := imsClient.ListImages(request)
	if err != nil {
		return volumeSize, err
	}

	if response.Images == nil || len(*response.Images) == 0 {
		return volumeSize, fmt.Errorf("can not find the image %s", imageID)
	}

	all := *response.Images
	volumeSize = int(all[0].MinDisk)
	log.Printf("[DEBUG] the minimum target volume size of image %s is %d GB", imageID, volumeSize)
	return volumeSize, nil
}
