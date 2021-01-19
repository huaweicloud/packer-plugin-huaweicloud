package huaweicloud

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
	"github.com/huaweicloud/golangsdk"
	"github.com/huaweicloud/golangsdk/openstack/compute/v2/extensions/startstop"
	"github.com/huaweicloud/golangsdk/openstack/compute/v2/servers"
)

type StepStopServer struct{}

func (s *StepStopServer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	server := state.Get("server").(*servers.Server)

	// We need the v2 compute client
	client, err := config.computeV2Client()
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Stopping server: %s ...", server.ID))
	if err := startstop.Stop(client, server.ID).ExtractErr(); err != nil {
		if errCode, ok := err.(golangsdk.ErrUnexpectedResponseCode); ok {
			if errCode.Actual == 409 {
				// The server might have already been shut down by Windows Sysprep
				log.Printf("[WARN] 409 on stopping an already stopped server, continuing")
				return multistep.ActionContinue
			}
		}

		err = fmt.Errorf("Error stopping server: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Message(fmt.Sprintf("Waiting for server to stop: %s ...", server.ID))
	stateChange := ServerStateChangeConf{
		Pending:   []string{"ACTIVE"},
		Target:    []string{"SHUTOFF", "STOPPED"},
		Refresh:   ServerStateRefreshFunc(client, server.ID),
		StepState: state,
	}
	if _, err := stateChange.WaitForState(); err != nil {
		err := fmt.Errorf("Error waiting for server (%s) to stop: %s", server.ID, err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	return multistep.ActionContinue
}

func (s *StepStopServer) Cleanup(state multistep.StateBag) {}
