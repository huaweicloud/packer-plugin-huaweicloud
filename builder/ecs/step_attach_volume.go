package ecs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	ecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
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

type StepAttachVolume struct {
	DataVolumes []DataVolume
}

type DataVolumeWrap struct {
	DataVolume
	dataType DataVolumeType
}

func (s *StepAttachVolume) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	if len(s.DataVolumes) == 0 {
		return multistep.ActionContinue
	}

	var dataVolumeWraps []DataVolumeWrap
	var err error
	if dataVolumeWraps, err = parseDataVolumes(s.DataVolumes); err != nil {
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	region := config.Region
	ecsClient, err := config.HcEcsClient(region)
	if err != nil {
		err = fmt.Errorf("error initializing ECS client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}
	evsClient, err := config.HcEvsClient(region)
	if err != nil {
		err = fmt.Errorf("error initializing EVS client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}
	imsClient, err := config.HcImsClient(region)
	if err != nil {
		err = fmt.Errorf("error initializing IMS client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	serverId := state.Get("server_id").(string)
	attachVolumeIds := make([]string, 0)
	for _, dataVolumeWrap := range dataVolumeWraps {
		switch dataVolumeWrap.dataType {
		case VolumeId:
			err = attachDataVolumes(ui, state, ecsClient, serverId, dataVolumeWrap.DataVolume)
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
			attachVolumeIds = append(attachVolumeIds, dataVolumeWrap.VolumeId)
		case Size:
			err = createVolume(ui, state, evsClient, serverId, dataVolumeWrap.DataVolume)
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
		case DataImageId:
			err = createVolumeByImageId(ui, state, imsClient, evsClient, serverId, dataVolumeWrap.DataVolume)
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
		case SnapshotId:
			err = createVolumeBySnapshotId(ui, state, evsClient, serverId, dataVolumeWrap.DataVolume)
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
		}
		state.Put("attach_volume_ids", attachVolumeIds)
	}
	return multistep.ActionContinue
}

func (s *StepAttachVolume) Cleanup(state multistep.StateBag) {
}

func parseDataVolumes(dataVolumes []DataVolume) ([]DataVolumeWrap, error) {
	dataVolumeWraps := make([]DataVolumeWrap, 0, len(dataVolumes))
	for _, dataVolume := range dataVolumes {
		specified := make([]string, 0)
		if dataVolume.Type == "" {
			dataVolume.Type = "SSD"
		}
		if dataVolume.VolumeId != "" {
			specified = append(specified, "volume_id")
			dataVolumeWraps = append(dataVolumeWraps, DataVolumeWrap{DataVolume: dataVolume, dataType: VolumeId})
		}
		if dataVolume.Size > 0 {
			specified = append(specified, "volume_size")
			dataVolumeWraps = append(dataVolumeWraps, DataVolumeWrap{DataVolume: dataVolume, dataType: Size})
		}
		if dataVolume.DataImageId != "" {
			specified = append(specified, "data_image_id")
			dataVolumeWraps = append(dataVolumeWraps, DataVolumeWrap{DataVolume: dataVolume, dataType: DataImageId})
		}
		if dataVolume.SnapshotId != "" {
			specified = append(specified, "snapshot_id")
			dataVolumeWraps = append(dataVolumeWraps, DataVolumeWrap{DataVolume: dataVolume, dataType: SnapshotId})
		}
		if len(specified) == 0 {
			return nil, fmt.Errorf("one of volume_id, volume_size, data_image_id, snapshot_id must be specified")
		}
		if len(specified) > 1 {
			return nil, fmt.Errorf("only one of volume_id, volume_size, data_image_id, snapshot_id can be"+
				"specified, but `%s` were specified", strings.Join(specified, ","))
		}
	}
	return dataVolumeWraps, nil
}

func attachDataVolumes(ui packer.Ui, state multistep.StateBag, ecsClient *ecs.EcsClient, serverId string, dataVolume DataVolume) error {
	ui.Say(fmt.Sprintf("Attaching volume to ECS..."))
	serverBody := &ecsmodel.AttachServerVolumeOption{
		VolumeId: dataVolume.VolumeId,
	}
	request := &ecsmodel.AttachServerVolumeRequest{
		ServerId: serverId,
		Body: &ecsmodel.AttachServerVolumeRequestBody{
			VolumeAttachment: serverBody,
		},
	}
	response, err := ecsClient.AttachServerVolume(request)
	if err != nil {
		return err
	}

	var jobID string

	if response.JobId != nil {
		jobID = *response.JobId
	}

	_, err = WaitForAttachVolumeJobSuccess(ui, state, ecsClient, jobID)
	if err != nil {
		return err
	}
	return nil
}

func createVolumeByImageId(ui packer.Ui, state multistep.StateBag, imsClient *ims.ImsClient, evsClient *evs.EvsClient,
	serverId string, dataVolume DataVolume) error {
	log.Printf("[DEBUG] Getting volume size by data image id: %s", dataVolume.DataImageId)

	request := &imsmodel.GlanceShowImageRequest{
		ImageId: dataVolume.DataImageId,
	}
	response, err := imsClient.GlanceShowImage(request)
	if err != nil {
		return err
	}
	dataVolume.Size = int(*response.MinDisk)
	return createVolume(ui, state, evsClient, serverId, dataVolume)
}

func createVolumeBySnapshotId(ui packer.Ui, state multistep.StateBag, evsClient *evs.EvsClient, serverId string,
	dataVolume DataVolume) error {
	log.Printf("[DEBUG] Getting volume size by snapshot id: %s", dataVolume.SnapshotId)

	request := &evsmodel.ShowSnapshotRequest{
		SnapshotId: dataVolume.SnapshotId,
	}
	response, err := evsClient.ShowSnapshot(request)
	if err != nil {
		return err
	}
	dataVolume.Size = int(*response.Snapshot.Size)
	return createVolume(ui, state, evsClient, serverId, dataVolume)
}

func createVolume(ui packer.Ui, state multistep.StateBag, evsClient *evs.EvsClient, serverId string, dataVolume DataVolume) error {
	ui.Say(fmt.Sprintf("Creating volume..."))

	config := state.Get("config").(*Config)
	availabilityZone := state.Get("availability_zone").(string)
	var volumeType evsmodel.CreateVolumeOptionVolumeType
	err := volumeType.UnmarshalJSON([]byte(dataVolume.Type))
	if err != nil {
		return fmt.Errorf("error parsing the data volume type %s: %s", dataVolume.Type, err)
	}
	serverBody := &evsmodel.CreateVolumeOption{
		AvailabilityZone: availabilityZone,
		VolumeType:       volumeType,
		Size:             int32(dataVolume.Size),
	}
	if config.EnterpriseProjectId != "" {
		serverBody.EnterpriseProjectId = &config.EnterpriseProjectId
	}
	if dataVolume.DataImageId != "" {
		serverBody.ImageRef = &dataVolume.DataImageId
	}
	if dataVolume.SnapshotId != "" {
		serverBody.SnapshotId = &dataVolume.SnapshotId
	}
	request := &evsmodel.CreateVolumeRequest{
		Body: &evsmodel.CreateVolumeRequestBody{
			Volume:   serverBody,
			ServerId: &serverId,
		},
	}
	response, err := evsClient.CreateVolume(request)
	if err != nil {
		return err
	}

	var jobID string

	if response.JobId != nil {
		jobID = *response.JobId
	}

	_, err = WaitForCreateVolumeJobSuccess(ui, state, evsClient, jobID)
	if err != nil {
		return err
	}
	return nil
}

func WaitForAttachVolumeJobSuccess(ui packer.Ui, state multistep.StateBag, client *ecs.EcsClient, jobID string) (*ecsmodel.ShowJobResponse, error) {
	ui.Message("Waiting for attach volume to ECS success...")
	stateChange := StateChangeConf{
		Pending:      []string{"INIT", "RUNNING"},
		Target:       []string{"SUCCESS"},
		Refresh:      serverJobStateRefreshFunc(client, jobID),
		Timeout:      10 * time.Minute,
		Delay:        10 * time.Second,
		PollInterval: 10 * time.Second,
		StateBag:     state,
	}
	serverJob, err := stateChange.WaitForState()
	if err != nil {
		err = fmt.Errorf("error waiting for volume (%s) to become ready: %s", jobID, err)
		ui.Error(err.Error())
		return nil, err
	}

	return serverJob.(*ecsmodel.ShowJobResponse), nil
}

func WaitForCreateVolumeJobSuccess(ui packer.Ui, state multistep.StateBag, client *evs.EvsClient, jobID string) (*evsmodel.ShowJobResponse, error) {
	ui.Message("Waiting for create volume success...")
	stateChange := StateChangeConf{
		Pending:      []string{"INIT", "RUNNING"},
		Target:       []string{"SUCCESS"},
		Refresh:      volumeJobStateRefreshFunc(client, jobID),
		Timeout:      10 * time.Minute,
		Delay:        10 * time.Second,
		PollInterval: 10 * time.Second,
		StateBag:     state,
	}
	serverJob, err := stateChange.WaitForState()
	if err != nil {
		err = fmt.Errorf("error waiting for create volume (%s) to become ready: %s", jobID, err)
		ui.Error(err.Error())
		return nil, err
	}

	return serverJob.(*evsmodel.ShowJobResponse), nil
}
