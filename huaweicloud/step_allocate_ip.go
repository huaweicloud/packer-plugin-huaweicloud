package huaweicloud

import (
	"context"
	"fmt"
	"time"

	"github.com/huaweicloud/golangsdk/openstack/compute/v2/servers"
	"github.com/huaweicloud/golangsdk/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
	"github.com/huaweicloud/golangsdk"
	"github.com/huaweicloud/golangsdk/openstack/networking/v1/eips"
)

type StepAllocateIp struct {
	FloatingIPNetwork     string
	FloatingIP            string
	ReuseIPs              bool
	InstanceFloatingIPNet string
	EIPType               string
	EIPBandwidthSize      int
}

func (s *StepAllocateIp) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	server := state.Get("server").(*servers.Server)

	var instanceIP floatingips.FloatingIP

	// This is here in case we error out before putting instanceIp into the
	// statebag below, because it is requested by Cleanup()
	state.Put("access_ip", &instanceIP)

	if (*s) == (StepAllocateIp{InstanceFloatingIPNet: s.InstanceFloatingIPNet}) {
		ui.Message("Floating IP not required")
		return multistep.ActionContinue
	}

	// We need the v2 compute client
	computeClient, err := config.computeV2Client()
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// We need the v2 network client
	networkClient, err := config.networkV2Client()
	if err != nil {
		err = fmt.Errorf("Error initializing network client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Try to Use the OpenStack floating IP by checking provided parameters in
	// the following order:
	//  - try to use "FloatingIP" ID directly if it's provided
	//  - try to find free floating IP in the project if "ReuseIPs" is set
	//  - create a new floating IP if "FloatingIPNetwork" is provided (it can be
	//    ID or name of the network).
	if s.FloatingIP != "" {
		// Try to use FloatingIP if it was provided by the user.
		freeFloatingIP, err := CheckFloatingIP(networkClient, s.FloatingIP)
		if err != nil {
			err := fmt.Errorf("Error using provided floating IP '%s': %s", s.FloatingIP, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		instanceIP = *freeFloatingIP
		ui.Message(fmt.Sprintf("Selected floating IP: '%s' (%s)", instanceIP.ID, instanceIP.FloatingIP))
		state.Put("floatingip_istemp", false)
	} else if s.ReuseIPs {
		// If ReuseIPs is set to true and we have a free floating IP, use it rather
		// than creating one.
		ui.Say(fmt.Sprint("Searching for unassociated floating IP"))
		freeFloatingIP, err := FindFreeFloatingIP(networkClient)
		if err != nil {
			err := fmt.Errorf("Error searching for floating IP: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		instanceIP = *freeFloatingIP
		ui.Message(fmt.Sprintf("Selected floating IP: '%s' (%s)", instanceIP.ID, instanceIP.FloatingIP))
		state.Put("floatingip_istemp", false)
	} else if s.FloatingIPNetwork != "" {
		// Lastly, if FloatingIPNetwork was provided by the user, we need to use it
		// to allocate a new floating IP and associate it to the instance.
		floatingNetwork, err := CheckFloatingIPNetwork(networkClient, s.FloatingIPNetwork)
		if err != nil {
			err := fmt.Errorf("Error using the provided floating_ip_network: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		ui.Say(fmt.Sprintf("Creating floating IP using network %s ...", floatingNetwork))
		newIP, err := floatingips.Create(networkClient, floatingips.CreateOpts{
			FloatingNetworkID: floatingNetwork,
		}).Extract()
		if err != nil {
			err := fmt.Errorf("Error creating floating IP from floating network '%s': %s", floatingNetwork, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		instanceIP = *newIP
		ui.Message(fmt.Sprintf("Created floating IP: '%s' (%s)", instanceIP.ID, instanceIP.FloatingIP))
		state.Put("floatingip_istemp", true)
	} else if (s.EIPType != "") && (s.EIPBandwidthSize != 0) {
		instanceIP, err = s.createEIP(ui, config, state)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
	}

	// Assoctate a floating IP if it was obtained in the previous steps.
	if instanceIP.ID != "" {
		ui.Say(fmt.Sprintf("Associating floating IP '%s' (%s) with instance port...",
			instanceIP.ID, instanceIP.FloatingIP))

		portID, err := GetInstancePortID(computeClient, server.ID, s.InstanceFloatingIPNet)
		if err != nil {
			err := fmt.Errorf("Error getting interfaces of the instance '%s': %s", server.ID, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		_, err = floatingips.Update(networkClient, instanceIP.ID, floatingips.UpdateOpts{
			PortID: &portID,
		}).Extract()
		if err != nil {
			err := fmt.Errorf(
				"Error associating floating IP '%s' (%s) with instance port '%s': %s",
				instanceIP.ID, instanceIP.FloatingIP, portID, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		ui.Message(fmt.Sprintf(
			"Added floating IP '%s' (%s) to instance!", instanceIP.ID, instanceIP.FloatingIP))
	}

	state.Put("access_ip", &instanceIP)
	return multistep.ActionContinue
}

func (s *StepAllocateIp) Cleanup(state multistep.StateBag) {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)
	instanceIP := state.Get("access_ip").(*floatingips.FloatingIP)

	// Don't clean up if unless required
	if instanceIP.ID == "" && instanceIP.FloatingIP == "" {
		return
	}

	// Don't delete pool addresses we didn't allocate
	if state.Get("floatingip_istemp") == false {
		return
	}

	// We need the v2 network client
	client, err := config.networkV2Client()
	if err != nil {
		ui.Error(fmt.Sprintf(
			"Error deleting temporary floating IP '%s' (%s)", instanceIP.ID, instanceIP.FloatingIP))
		return
	}

	if instanceIP.ID != "" {
		if err := floatingips.Delete(client, instanceIP.ID).ExtractErr(); err != nil {
			ui.Error(fmt.Sprintf(
				"Error deleting temporary floating IP '%s' (%s)", instanceIP.ID, instanceIP.FloatingIP))
			return
		}

		ui.Say(fmt.Sprintf("Deleted temporary floating IP '%s' (%s)", instanceIP.ID, instanceIP.FloatingIP))
	}
}

func (s *StepAllocateIp) createEIP(ui packer.Ui, config *Config, stateBag multistep.StateBag) (floatingips.FloatingIP, error) {
	ui.Say(fmt.Sprintf("Creating EIP ..."))

	result := floatingips.FloatingIP{}
	client, err := config.networkV1Client()
	if err != nil {
		err = fmt.Errorf("Error initializing vpc client: %s", err)
		ui.Error(err.Error())
		return result, err
	}

	createOpts := eips.ApplyOpts{
		IP: eips.PublicIpOpts{
			Type: s.EIPType,
		},
		Bandwidth: eips.BandwidthOpts{
			Size:       s.EIPBandwidthSize,
			ShareType:  "PER",
			ChargeMode: "bandwidth",
			Name:       fmt.Sprintf("packer_eip_bandwidth_%v", time.Now().Unix()),
		},
	}
	eip, err := eips.Apply(client, createOpts).Extract()
	if err != nil {
		err = fmt.Errorf("Error creating EIP: %s", err)
		ui.Error(err.Error())
		return result, err
	}
	ui.Message(fmt.Sprintf("Created EIP: '%s' (%s)", eip.ID, eip.PublicAddress))

	stateConf := &StateChangeConf1{
		Target:     []string{"ACTIVE"},
		Refresh:    getEIPStatus(client, eip.ID),
		Timeout:    10 * time.Minute,
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
		StateBag:   stateBag,
	}
	_, err = stateConf.WaitForState()
	if err != nil {
		err = fmt.Errorf("Error waiting eip to be active: %s", err)
		ui.Error(err.Error())
		return result, err
	}

	result.ID = eip.ID
	result.FloatingIP = eip.PublicAddress
	return result, nil
}

func getEIPStatus(client *golangsdk.ServiceClient, eipID string) StateRefreshFunc1 {
	return func() (interface{}, string, error) {
		e, err := eips.Get(client, eipID).Extract()
		if err != nil {
			return nil, "", nil
		}

		if e.Status == "DOWN" || e.Status == "ACTIVE" {
			return e, "ACTIVE", nil
		}

		return e, "", nil
	}
}
