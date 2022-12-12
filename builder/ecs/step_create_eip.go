package ecs

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	eip "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2/model"
)

type StepCreatePublicipIP struct {
	PublicipIP       string
	ReuseIPs         bool
	EIPType          string
	EIPBandwidthSize int
	doCleanup        bool
}

type PublicipIP struct {
	ID      string
	Address string
}

func (s *StepCreatePublicipIP) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	var accessEIP PublicipIP

	// This is here in case we error out before putting accessEIP into the
	// statebag below, because it is requested by Cleanup()
	state.Put("access_eip", &accessEIP)

	region := config.Region
	eipClient, err := config.HcEipClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing EIP client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Try to Use the public IP by checking provided parameters in
	// the following order:
	//  - try to use "PublicipIP" ID directly if it's provided
	//  - try to find free public IP in the project if "ReuseIPs" is set
	//  - create a new public IP if "EIPType" and "EIPBandwidthSize" are provided.
	if s.PublicipIP != "" {
		ui.Say(fmt.Sprintf("Checking the provided public IP %s ...", s.PublicipIP))
		freeFloatingIP, err := checkPublicIP(eipClient, s.PublicipIP)
		if err != nil {
			err := fmt.Errorf("Error using provided public IP '%s': %s", s.PublicipIP, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		accessEIP = *freeFloatingIP
		ui.Message(fmt.Sprintf("Selected public IP: '%s' (%s)", accessEIP.ID, accessEIP.Address))
		s.doCleanup = false
	} else if s.ReuseIPs {
		// If ReuseIPs is set to true and we have a free public IP, use it rather
		// than creating one.
		ui.Say(fmt.Sprint("Searching for unassociated public IP ..."))
		freeFloatingIP, err := findFreePublicIP(eipClient)
		if err != nil {
			err := fmt.Errorf("Error searching for public IP: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		accessEIP = *freeFloatingIP
		ui.Message(fmt.Sprintf("Selected public IP: '%s' (%s)", accessEIP.ID, accessEIP.Address))
		s.doCleanup = false
	} else if s.EIPBandwidthSize != 0 {
		if s.EIPType == "" {
			s.EIPType = "5_bgp"
		}

		accessEIP, err = s.createEIP(ui, config, state)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
		s.doCleanup = true
	}

	state.Put("access_eip", &accessEIP)
	return multistep.ActionContinue
}

func (s *StepCreatePublicipIP) Cleanup(state multistep.StateBag) {
	if !s.doCleanup {
		return
	}

	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	accessEIP := state.Get("access_eip").(*PublicipIP)
	if accessEIP.ID == "" || accessEIP.Address == "" {
		return
	}

	region := config.Region
	eipClient, err := config.HcEipClient(region)
	if err != nil {
		ui.Error(fmt.Sprintf(
			"Error deleting temporary public IP '%s' (%s)", accessEIP.ID, accessEIP.Address))
		return
	}

	if accessEIP.ID != "" {
		request := &model.DeletePublicipRequest{
			PublicipId: accessEIP.ID,
		}
		if _, err := eipClient.DeletePublicip(request); err != nil {
			ui.Error(fmt.Sprintf(
				"Error deleting temporary public IP '%s' (%s)", accessEIP.ID, accessEIP.Address))
			return
		}

		ui.Say(fmt.Sprintf("Deleted temporary public IP '%s' (%s)", accessEIP.ID, accessEIP.Address))
	}
}

func (s *StepCreatePublicipIP) createEIP(ui packer.Ui, config *Config, stateBag multistep.StateBag) (PublicipIP, error) {
	result := PublicipIP{}
	ui.Say(fmt.Sprintf("Creating EIP ..."))

	region := config.Region
	eipClient, err := config.HcEipClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing EIP client: %s", err)
		ui.Error(err.Error())
		return result, err
	}

	bwdName := fmt.Sprintf("packer_eip_bandwidth_%v", time.Now().Unix())
	bwdSize := int32(s.EIPBandwidthSize)
	chargeMode := model.GetCreatePublicipBandwidthOptionChargeModeEnum().TRAFFIC
	bandwidthOpts := model.CreatePublicipBandwidthOption{
		Name:       &bwdName,
		Size:       &bwdSize,
		ChargeMode: &chargeMode,
		ShareType:  model.GetCreatePublicipBandwidthOptionShareTypeEnum().PER,
	}

	publicipOpts := model.CreatePublicipOption{
		Type: s.EIPType,
	}
	request := &model.CreatePublicipRequest{
		Body: &model.CreatePublicipRequestBody{
			Publicip:  &publicipOpts,
			Bandwidth: &bandwidthOpts,
		},
	}
	response, err := eipClient.CreatePublicip(request)
	if err != nil {
		err = fmt.Errorf("Error creating EIP: %s", err)
		ui.Error(err.Error())
		return result, err
	}
	if response.Publicip == nil {
		return result, fmt.Errorf("failed to obtain the EIP details")
	}

	eipID := *response.Publicip.Id
	ui.Message(fmt.Sprintf("Created EIP: '%s' (%s)", eipID, *response.Publicip.PublicIpAddress))

	stateConf := &StateChangeConf{
		Pending:    []string{"PENDING"},
		Target:     []string{"ACTIVE"},
		Refresh:    getEIPStatus(eipClient, eipID),
		Timeout:    5 * time.Minute,
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
		StateBag:   stateBag,
	}
	state, err := stateConf.WaitForState()
	if err != nil {
		err = fmt.Errorf("Error waiting eip to be active: %s", err)
		ui.Error(err.Error())
		return result, err
	}

	result = state.(PublicipIP)
	return result, nil
}

func getEIPStatus(client *eip.EipClient, eipID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		request := &model.ShowPublicipRequest{
			PublicipId: eipID,
		}
		response, err := client.ShowPublicip(request)
		if err != nil {
			return nil, "", err
		}

		if response.Publicip == nil {
			return nil, "", nil
		}

		object := response.Publicip
		result := PublicipIP{
			ID:      *object.Id,
			Address: *object.PublicIpAddress,
		}
		status := object.Status.Value()
		if status == "DOWN" || status == "ACTIVE" {
			return result, "ACTIVE", nil
		}

		return result, "PENDING", nil
	}
}

// checkPublicipIP gets a public IP by its ID and checks if it is already
// associated with any internal interface.
// It returns public IP if it can be used.
func checkPublicIP(client *eip.EipClient, id string) (*PublicipIP, error) {
	request := &model.ShowPublicipRequest{
		PublicipId: id,
	}
	response, err := client.ShowPublicip(request)
	if err != nil {
		return nil, err
	}

	object := response.Publicip
	if object == nil {
		return nil, fmt.Errorf("failed to obtain the EIP details")
	}

	if object.PortId != nil && *object.PortId != "" {
		return nil, fmt.Errorf("the provided public IP '%s' is already associated with port '%s'",
			id, *object.PortId)
	}

	result := PublicipIP{
		ID:      *object.Id,
		Address: *object.PublicIpAddress,
	}
	return &result, nil
}

var LimitCount int32 = 50

// findFreePublicipIP returns free unassociated public IP.
// It will return first public IP if there are many.
func findFreePublicIP(client *eip.EipClient) (*PublicipIP, error) {
	var freePublicipIP *PublicipIP

	var marker *string
	for {
		request := &model.ListPublicipsRequest{
			Marker: marker,
			Limit:  &LimitCount,
		}
		response, err := client.ListPublicips(request)
		if err != nil {
			return nil, err
		}

		if response.Publicips == nil || len(*response.Publicips) == 0 {
			break
		}

		for _, item := range *response.Publicips {
			marker = item.Id
			// the public IP is associated with port
			if item.PortId != nil && *item.PortId != "" {
				continue
			}

			// the public IP is able to be allocated
			freePublicipIP = &PublicipIP{
				ID:      *item.Id,
				Address: *item.PublicIpAddress,
			}
			break
		}

		if freePublicipIP != nil {
			return freePublicipIP, nil
		}

		// it's the last page
		total := len(*response.Publicips)
		if int32(total) < LimitCount {
			break
		}
	}

	return nil, fmt.Errorf("no free public IPs found")
}
