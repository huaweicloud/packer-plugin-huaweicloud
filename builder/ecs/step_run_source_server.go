package ecs

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/huaweicloud/golangsdk"
	"github.com/huaweicloud/golangsdk/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/huaweicloud/golangsdk/openstack/compute/v2/extensions/keypairs"
	"github.com/huaweicloud/golangsdk/openstack/compute/v2/servers"
)

type StepRunSourceServer struct {
	Name             string
	VpcID            string
	Subnets          []string
	SecurityGroups   []string
	AvailabilityZone string
	UserData         string
	UserDataFile     string
	ConfigDrive      bool
	InstanceMetadata map[string]string
	server           *servers.Server
}

func (s *StepRunSourceServer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	flavor := state.Get("flavor_id").(string)
	sourceImage := state.Get("source_image").(string)
	ui := state.Get("ui").(packer.Ui)

	// We need the v2 compute client
	computeClient, err := config.computeV2Client()
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	networks, err := s.getNetworks(config)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	userData := []byte(s.UserData)
	if s.UserDataFile != "" {
		userData, err = ioutil.ReadFile(s.UserDataFile)
		if err != nil {
			err = fmt.Errorf("Error reading user data file: %s", err)
			state.Put("error", err)
			return multistep.ActionHalt
		}
	}

	availabilityZone := state.Get("availability_zone").(string)
	serverOpts := servers.CreateOpts{
		Name:             s.Name,
		ImageRef:         sourceImage,
		FlavorRef:        flavor,
		SecurityGroups:   s.SecurityGroups,
		Networks:         networks,
		AvailabilityZone: availabilityZone,
		UserData:         userData,
		ConfigDrive:      &s.ConfigDrive,
		ServiceClient:    computeClient,
		Metadata:         s.InstanceMetadata,
	}

	var serverOptsExt servers.CreateOptsBuilder

	// Create root volume in the Block Storage service,
	// Add block device mapping v2 to the server create options
	volume := state.Get("volume_id").(string)
	blockDeviceMappingV2 := []bootfromvolume.BlockDevice{
		{
			BootIndex:       0,
			DestinationType: bootfromvolume.DestinationVolume,
			SourceType:      bootfromvolume.SourceVolume,
			UUID:            volume,
		},
	}
	// ImageRef and block device mapping is an invalid options combination.
	serverOpts.ImageRef = ""
	serverOptsExt = bootfromvolume.CreateOptsExt{
		CreateOptsBuilder: &serverOpts, // must pass pointer, because it will be changed later
		BlockDevice:       blockDeviceMappingV2,
	}

	// Add keypair to the server create options.
	keyName := config.Comm.SSHKeyPairName
	if keyName != "" {
		serverOptsExt = keypairs.CreateOptsExt{
			CreateOptsBuilder: serverOptsExt,
			KeyName:           keyName,
		}
	}

	ui.Say(fmt.Sprintf("Launching server in az:%s ...", serverOpts.AvailabilityZone))
	server, err := createServer(ui, state, computeClient, serverOptsExt)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	s.server = server
	state.Put("server", server)

	return multistep.ActionContinue
}

func (s *StepRunSourceServer) Cleanup(state multistep.StateBag) {
	if s.server == nil {
		return
	}

	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)

	// We need the v2 compute client
	computeClient, err := config.computeV2Client()
	if err != nil {
		ui.Error(fmt.Sprintf("Error terminating server, may still be around: %s", err))
		return
	}

	ui.Say(fmt.Sprintf("Terminating the source server: %s ...", s.server.ID))
	if err := servers.Delete(computeClient, s.server.ID).ExtractErr(); err != nil {
		ui.Error(fmt.Sprintf("Error terminating server, may still be around: %s", err))
		return
	}

	stateChange := ServerStateChangeConf{
		Pending: []string{"ACTIVE", "BUILD", "REBUILD", "SUSPENDED", "SHUTOFF", "STOPPED"},
		Refresh: ServerStateRefreshFunc(computeClient, s.server.ID),
		Target:  []string{"DELETED"},
	}

	stateChange.WaitForState()
}

func createServer(ui packer.Ui, state multistep.StateBag, client *golangsdk.ServiceClient, opts servers.CreateOptsBuilder) (*servers.Server, error) {
	server, err := servers.Create(client, opts).Extract()
	if err != nil {
		err = fmt.Errorf("Error launching source server: %s", err)
		ui.Error(err.Error())
		return nil, err
	}

	ui.Message(fmt.Sprintf("Server ID: %s", server.ID))
	log.Printf("server id: %s", server.ID)

	ui.Say("Waiting for server to become ready...")
	stateChange := ServerStateChangeConf{
		Pending:   []string{"BUILD"},
		Target:    []string{"ACTIVE"},
		Refresh:   ServerStateRefreshFunc(client, server.ID),
		StepState: state,
	}
	latestServer, err := stateChange.WaitForState()
	if err != nil {
		err = fmt.Errorf("Error waiting for server (%s) to become ready: %s", server.ID, err)
		ui.Error(err.Error())
		return nil, err
	}

	return latestServer.(*servers.Server), nil
}

func (s *StepRunSourceServer) getNetworks(config *Config) ([]servers.Network, error) {
	if s.VpcID == "" {
		return nil, nil
	}

	networks := make([]servers.Network, len(s.Subnets))
	for i, id := range s.Subnets {
		networks[i] = servers.Network{
			UUID: id,
		}
	}

	return networks, nil
}
