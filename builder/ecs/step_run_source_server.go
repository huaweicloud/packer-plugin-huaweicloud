package ecs

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
)

type StepRunSourceServer struct {
	Name             string
	VpcID            string
	Subnets          []string
	SecurityGroups   []string
	AvailabilityZone string
	RootVolumeType   string
	RootVolumeSize   int
	UserData         string
	UserDataFile     string
	ConfigDrive      bool
	InstanceMetadata map[string]string
	serverID         string
}

func (s *StepRunSourceServer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	flavor := state.Get("flavor_id").(string)
	sourceImage := state.Get("source_image").(string)

	region := config.Region
	ecsClient, err := config.HcEcsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	vpcID := state.Get("vpc_id").(string)
	networks := s.buildNetworks(state)
	secGroups := s.buildSecurityGroups()
	publicIP := s.buildPublicIP(state)

	rootVolume, err := s.buildRootVolume()
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	userData := s.UserData
	if s.UserDataFile != "" {
		rawData, err := ioutil.ReadFile(s.UserDataFile)
		if err != nil {
			err = fmt.Errorf("Error reading user data file: %s", err)
			state.Put("error", err)
			return multistep.ActionHalt
		}
		userData = string(rawData)
	}

	availabilityZone := state.Get("availability_zone").(string)
	keyName := config.Comm.SSHKeyPairName

	serverbody := &model.PostPaidServer{
		Name:             s.Name,
		ImageRef:         sourceImage,
		FlavorRef:        flavor,
		KeyName:          &keyName,
		Vpcid:            vpcID,
		Nics:             networks,
		SecurityGroups:   &secGroups,
		AvailabilityZone: &availabilityZone,
		RootVolume:       rootVolume,
		Publicip:         publicIP,
		UserData:         &userData,
		Metadata:         s.InstanceMetadata,
	}
	request := &model.CreatePostPaidServersRequest{
		Body: &model.CreatePostPaidServersRequestBody{
			Server: serverbody,
		},
	}

	ui.Say(fmt.Sprintf("Launching server in AZ %s...", availabilityZone))
	response, err := ecsClient.CreatePostPaidServers(request)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ids := *response.ServerIds
	jobID := *response.JobId
	serverJob, err := WaitForServerJobSuccess(ui, state, ecsClient, jobID)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var serverID string
	if len(ids) > 0 {
		serverID = ids[0]
	} else if serverJob.Entities != nil && len(*serverJob.Entities.SubJobs) > 0 {
		subJobs := *serverJob.Entities.SubJobs
		if len(subJobs) > 0 && subJobs[0].Entities != nil {
			serverID = *subJobs[0].Entities.ServerId
		}
	}

	accessPrivateIP, err := getAccessPrivateIP(ecsClient, serverID)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Message(fmt.Sprintf("Server ID: %s", serverID))
	s.serverID = serverID

	state.Put("server_id", serverID)
	state.Put("access_private_ip", accessPrivateIP)

	return multistep.ActionContinue
}

// getAccessPrivateIP returns the first internal port of the instance that can be used for
// the association of a public IP.
func getAccessPrivateIP(client *ecs.EcsClient, serverID string) (string, error) {
	var primaryIP string
	request := &model.ListServerInterfacesRequest{
		ServerId: serverID,
	}
	response, err := client.ListServerInterfaces(request)
	if err != nil {
		return "", err
	}

	if response.InterfaceAttachments == nil || len(*response.InterfaceAttachments) == 0 {
		return "", fmt.Errorf("no interfaces attachmented")
	}

	allNics := *response.InterfaceAttachments
	for _, nic := range allNics {
		nicIPs := *nic.FixedIps
		if len(nicIPs) == 0 {
			continue
		}

		if nicIPs[0].IpAddress != nil {
			primaryIP = *nicIPs[0].IpAddress
			break
		}
	}

	if primaryIP == "" {
		return "", fmt.Errorf("no private address attachmented")
	}
	return primaryIP, nil
}

func (s *StepRunSourceServer) Cleanup(state multistep.StateBag) {
	if s.serverID == "" {
		return
	}

	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	region := config.Region
	ecsClient, err := config.HcEcsClient(region)
	if err != nil {
		ui.Error(fmt.Sprintf("Error terminating server, may still be around: %s", err))
		return
	}

	serverID := s.serverID
	ui.Say(fmt.Sprintf("Terminating the source server: %s...", serverID))

	serversbody := []model.ServerId{
		{
			Id: serverID,
		},
	}
	request := &model.DeleteServersRequest{
		Body: &model.DeleteServersRequestBody{
			Servers: serversbody,
		},
	}
	_, err = ecsClient.DeleteServers(request)
	if err != nil {
		ui.Error(fmt.Sprintf("Error terminating server, may still be around: %s", err))
		return
	}

	stateChange := StateChangeConf{
		Pending:      []string{"ACTIVE", "BUILD", "REBUILD", "SUSPENDED", "SHUTOFF", "STOPPED"},
		Target:       []string{"DELETED"},
		Refresh:      serverStateRefreshFunc(ecsClient, serverID),
		Timeout:      10 * time.Minute,
		Delay:        10 * time.Second,
		PollInterval: 10 * time.Second,
	}

	stateChange.WaitForState()
}

func WaitForServerJobSuccess(ui packer.Ui, state multistep.StateBag, client *ecs.EcsClient, jobID string) (*model.ShowJobResponse, error) {
	ui.Say("Waiting for server to become ready...")
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
		err = fmt.Errorf("Error waiting for server (%s) to become ready: %s", jobID, err)
		ui.Error(err.Error())
		return nil, err
	}

	return serverJob.(*model.ShowJobResponse), nil
}

func (s *StepRunSourceServer) buildNetworks(state multistep.StateBag) []model.PostPaidServerNic {
	vpcID := state.Get("vpc_id").(string)
	if vpcID == "" {
		return nil
	}

	subnets := state.Get("subnets").([]string)
	networks := make([]model.PostPaidServerNic, len(subnets))
	for i, id := range subnets {
		networks[i] = model.PostPaidServerNic{
			SubnetId: id,
		}
	}

	return networks
}

func (s *StepRunSourceServer) buildSecurityGroups() []model.PostPaidServerSecurityGroup {
	rawGroups := s.SecurityGroups
	if len(rawGroups) == 0 {
		return nil
	}

	secGroups := make([]model.PostPaidServerSecurityGroup, 0, len(rawGroups))
	for _, id := range rawGroups {
		if strings.Contains(id, "default") {
			continue
		}

		secGroups = append(secGroups, model.PostPaidServerSecurityGroup{
			Id: &id,
		})
	}

	return secGroups
}

func (s *StepRunSourceServer) buildPublicIP(state multistep.StateBag) *model.PostPaidServerPublicip {
	accessEIP := state.Get("access_eip").(*PublicipIP)
	if accessEIP == nil || accessEIP.ID == "" {
		return nil
	}

	publicIP := model.PostPaidServerPublicip{
		Id: &accessEIP.ID,
	}
	return &publicIP
}

func (s *StepRunSourceServer) buildRootVolume() (*model.PostPaidServerRootVolume, error) {
	if s.RootVolumeType == "" {
		s.RootVolumeType = "SSD"
	}

	var volumeType model.PostPaidServerRootVolumeVolumetype
	err := volumeType.UnmarshalJSON([]byte(s.RootVolumeType))
	if err != nil {
		return nil, fmt.Errorf("Error parsing the root volume type %s: %s", s.RootVolumeType, err)
	}

	rootVolume := model.PostPaidServerRootVolume{
		Volumetype: volumeType,
	}

	volumeSize := int32(s.RootVolumeSize)
	if volumeSize != 0 {
		rootVolume.Size = &volumeSize
	}

	return &rootVolume, nil
}
