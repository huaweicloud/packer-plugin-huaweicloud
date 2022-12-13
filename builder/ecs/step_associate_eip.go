package ecs

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/huaweicloud/golangsdk/openstack/compute/v2/servers"
	ecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2/model"
)

type StepAssociatePublicipIP struct{}

func (s *StepAssociatePublicipIP) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	accessEIP := state.Get("access_eip").(*PublicipIP)
	if accessEIP == nil || accessEIP.Address == "" {
		return multistep.ActionContinue
	}

	region := config.Region
	eipClient, err := config.HcEipClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing EIP client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	portID, err := getInstancePortID(state)
	if err != nil {
		err := fmt.Errorf("Error getting interfaces of the instance: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Associating public IP '%s' (%s) with instance port %s ...",
		accessEIP.ID, accessEIP.Address, portID))

	updateOpts := model.UpdatePublicipOption{
		PortId: &portID,
	}
	request := &model.UpdatePublicipRequest{
		PublicipId: accessEIP.ID,
		Body: &model.UpdatePublicipsRequestBody{
			Publicip: &updateOpts,
		},
	}
	_, err = eipClient.UpdatePublicip(request)
	if err != nil {
		err := fmt.Errorf(
			"Error associating public IP '%s' (%s) with instance port '%s': %s",
			accessEIP.ID, accessEIP.Address, portID, err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	ui.Message(fmt.Sprintf(
		"Added public IP '%s' (%s) to instance!", accessEIP.ID, accessEIP.Address))

	return multistep.ActionContinue
}

// getInstancePortID returns the first internal port of the instance that can be used for
// the association of a public IP.
func getInstancePortID(state multistep.StateBag) (string, error) {
	config := state.Get("config").(*Config)
	server := state.Get("server").(*servers.Server)

	region := config.Region
	ecsClient, err := config.HcEcsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing ECS client: %s", err)
		return "", err
	}

	request := &ecsmodel.ListServerInterfacesRequest{
		ServerId: server.ID,
	}
	response, err := ecsClient.ListServerInterfaces(request)
	if err != nil {
		return "", err
	}

	if response.InterfaceAttachments == nil || len(*response.InterfaceAttachments) == 0 {
		return "", fmt.Errorf("no interfaces attachmented")
	}

	allNics := *response.InterfaceAttachments
	return *allNics[0].PortId, nil
}

func (s *StepAssociatePublicipIP) Cleanup(state multistep.StateBag) {}
