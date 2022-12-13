package ecs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
)

// StepGetPassword reads the password from a booted HuaweiCloud server and sets
// it on the WinRM config.
type StepGetPassword struct {
	Debug     bool
	Comm      *communicator.Config
	BuildName string
}

func (s *StepGetPassword) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)

	// Skip if we're not using winrm
	if s.Comm.Type != "winrm" {
		log.Printf("[INFO] Not using winrm communicator, skipping get password...")
		return multistep.ActionContinue
	}

	// If we already have a password, skip it
	if s.Comm.WinRMPassword != "" {
		ui.Say("Skipping waiting for password since WinRM password set...")
		return multistep.ActionContinue
	}

	region := config.Region
	ecsClient, err := config.HcEcsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say("Waiting for password since WinRM password is not set...")
	serverID := state.Get("server_id").(string)

	var password string
	for password == "" {

		request := &model.ShowServerPasswordRequest{
			ServerId: serverID,
		}
		response, err := ecsClient.ShowServerPassword(request)
		if err != nil {
			err = fmt.Errorf("Error initializing compute client: %s", err)
			state.Put("error", err)
			return multistep.ActionHalt
		}

		password = *response.Password
		// Check for an interrupt in between attempts.
		if _, ok := state.GetOk(multistep.StateCancelled); ok {
			return multistep.ActionHalt
		}

		log.Printf("Retrying to get a administrator password evry 5 seconds.")
		time.Sleep(5 * time.Second)
	}

	ui.Message(fmt.Sprintf("Password retrieved!"))
	s.Comm.WinRMPassword = password

	// In debug-mode, we output the password
	if s.Debug {
		ui.Message(fmt.Sprintf(
			"Password (since debug is enabled) \"%s\"", s.Comm.WinRMPassword))
	}

	packer.LogSecretFilter.Set(s.Comm.WinRMPassword)

	return multistep.ActionContinue
}

func (s *StepGetPassword) Cleanup(multistep.StateBag) {}
