package ecs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
)

type StepStopServer struct{}

func (s *StepStopServer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	region := config.Region
	client, err := config.HcEcsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	serverID := state.Get("server_id").(string)
	ui.Say(fmt.Sprintf("Stopping server: %s ...", serverID))

	stopBody := &model.BatchStopServersOption{
		Servers: []model.ServerId{
			{
				Id: serverID,
			},
		},
	}

	request := &model.BatchStopServersRequest{
		Body: &model.BatchStopServersRequestBody{
			OsStop: stopBody,
		},
	}

	if _, err := client.BatchStopServers(request); err != nil {
		// we can make an image when the server is running or not, continue
		log.Printf("[WARN] failed to stop server: %s", err)
		return multistep.ActionContinue
	}

	ui.Message(fmt.Sprintf("Waiting for server to stop: %s ...", serverID))
	stateChange := StateChangeConf{
		Pending:      []string{"ACTIVE"},
		Target:       []string{"SHUTOFF", "STOPPED"},
		Refresh:      serverStateRefreshFunc(client, serverID),
		Timeout:      3 * time.Minute,
		Delay:        5 * time.Second,
		PollInterval: 5 * time.Second,
		StateBag:     state,
	}
	if _, err := stateChange.WaitForState(); err != nil {
		log.Printf("[WARN] error waiting for server (%s) to stop: %s", serverID, err)
	}

	return multistep.ActionContinue
}

func (s *StepStopServer) Cleanup(state multistep.StateBag) {}
