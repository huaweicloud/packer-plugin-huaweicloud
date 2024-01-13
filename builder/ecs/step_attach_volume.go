package ecs

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	ecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	evs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/evs/v2"
	evsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/evs/v2/model"
)

type StepAttachVolume struct {
	PrefixName string
}

func (s *StepAttachVolume) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	dataVolumes := state.Get("data_disk_wraps")
	if dataVolumes == nil {
		return multistep.ActionContinue
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

	attachVolumeIds := make([]string, 0)
	serverId := state.Get("server_id").(string)
	index := 1

	dataVolumeWraps := dataVolumes.([]DataVolumeWrap)
	for _, dataVolumeWrap := range dataVolumeWraps {
		switch dataVolumeWrap.dataType {
		case VolumeId:
			volumeId := dataVolumeWrap.VolumeId
			ui.Say(fmt.Sprintf("Attaching volume %s to ECS...", volumeId))

			dataVolumeWrap.serverId = serverId
			err = attachDataVolumes(ui, state, ecsClient, dataVolumeWrap)
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}

			ui.Message(fmt.Sprintf("Attached volume %s to ECS", volumeId))
			attachVolumeIds = append(attachVolumeIds, volumeId)
		case Size, DataImageId, SnapshotId:
			volumeName := s.generateVolumeName(index)
			ui.Say(fmt.Sprintf("Creating and attaching %s...", volumeName))

			dataVolumeWrap.serverId = serverId
			dataVolumeWrap.volumeName = volumeName
			err = createAndAttachVolume(ui, state, evsClient, dataVolumeWrap, index)
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}

			ui.Message(fmt.Sprintf("Attached volume %s to ECS", volumeName))
			index++
		}
		state.Put("attach_volume_ids", attachVolumeIds)
	}
	return multistep.ActionContinue
}

func (s *StepAttachVolume) Cleanup(state multistep.StateBag) {
}

func (s *StepAttachVolume) generateVolumeName(index int) string {
	return fmt.Sprintf("%s-volume-%04d", s.PrefixName, index)
}

func attachDataVolumes(ui packer.Ui, state multistep.StateBag, ecsClient *ecs.EcsClient, disk DataVolumeWrap) error {
	volumeId := disk.VolumeId
	attachBody := &ecsmodel.AttachServerVolumeOption{
		VolumeId: volumeId,
	}
	request := &ecsmodel.AttachServerVolumeRequest{
		ServerId: disk.serverId,
		Body: &ecsmodel.AttachServerVolumeRequestBody{
			VolumeAttachment: attachBody,
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

	_, err = waitForAttachVolumeJobSuccess(ui, state, ecsClient, jobID)
	if err != nil {
		return err
	}

	return nil
}

func createAndAttachVolume(ui packer.Ui, state multistep.StateBag, evsClient *evs.EvsClient, disk DataVolumeWrap, index int) error {
	config := state.Get("config").(*Config)
	availabilityZone := state.Get("availability_zone").(string)

	if disk.Type == "" {
		disk.Type = "SSD"
	}
	var volumeType evsmodel.CreateVolumeOptionVolumeType
	err := volumeType.UnmarshalJSON([]byte(disk.Type))
	if err != nil {
		return fmt.Errorf("error parsing the data volume type %s: %s", disk.Type, err)
	}

	volumeBody := &evsmodel.CreateVolumeOption{
		AvailabilityZone: availabilityZone,
		VolumeType:       volumeType,
		Size:             int32(disk.Size),
		Name:             &disk.volumeName,
	}
	if config.EnterpriseProjectId != "" {
		volumeBody.EnterpriseProjectId = &config.EnterpriseProjectId
	}
	if disk.DataImageId != "" {
		volumeBody.ImageRef = &disk.DataImageId
	}
	if disk.SnapshotId != "" {
		volumeBody.SnapshotId = &disk.SnapshotId
	}
	if disk.KmsKeyID != "" {
		volumeBody.Metadata = map[string]string{
			"__system__encrypted": "1",
			"__system__cmkid":     disk.KmsKeyID,
		}
	}

	request := &evsmodel.CreateVolumeRequest{
		Body: &evsmodel.CreateVolumeRequestBody{
			Volume:   volumeBody,
			ServerId: &disk.serverId,
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

	_, err = waitForCreateVolumeJobSuccess(ui, state, evsClient, jobID)
	if err != nil {
		return err
	}
	return nil
}

func waitForAttachVolumeJobSuccess(ui packer.Ui, state multistep.StateBag, client *ecs.EcsClient, jobID string) (*ecsmodel.ShowJobResponse, error) {
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

func waitForCreateVolumeJobSuccess(ui packer.Ui, state multistep.StateBag, client *evs.EvsClient, jobID string) (*evsmodel.ShowJobResponse, error) {
	ui.Message("Waiting for create volume success...")
	stateChange := StateChangeConf{
		Pending:      []string{"PENDING"},
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
