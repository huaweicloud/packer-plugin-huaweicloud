package ecs

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"

	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
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

	ui.Say("Waiting for password since WinRM password is not set...")
	privateKey, err := ssh.ParseRawPrivateKey(s.Comm.SSHPrivateKey)
	if err != nil {
		err = fmt.Errorf("Error parsing private key: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	region := config.Region
	ecsClient, err := config.HcEcsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	serverID := state.Get("server_id").(string)
	stateConf := &StateChangeConf{
		Pending:      []string{"PENDING"},
		Target:       []string{"SUCCESS"},
		Refresh:      getencryptedPassword(ecsClient, serverID),
		Timeout:      10 * time.Minute,
		Delay:        30 * time.Second,
		PollInterval: 10 * time.Second,
		StateBag:     state,
	}

	result, err := stateConf.WaitForState()
	if err != nil {
		err = fmt.Errorf("Error getting the encrypted password: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	encryptedPassword := result.(string)
	password, err := decryptPassword(encryptedPassword, privateKey.(*rsa.PrivateKey))
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
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

func getencryptedPassword(client *ecs.EcsClient, serverID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		request := &model.ShowServerPasswordRequest{
			ServerId: serverID,
		}

		response, err := client.ShowServerPassword(request)
		if err != nil {
			return "", "ERROR", err
		}

		password := *response.Password
		if password == "" {
			return "", "PENDING", nil
		}
		return password, "SUCCESS", nil
	}
}

func decryptPassword(encryptedPassword string, privateKey *rsa.PrivateKey) (string, error) {
	b64EncryptedPassword := make([]byte, base64.StdEncoding.DecodedLen(len(encryptedPassword)))

	n, err := base64.StdEncoding.Decode(b64EncryptedPassword, []byte(encryptedPassword))
	if err != nil {
		return "", fmt.Errorf("Failed to base64 decode encrypted password: %s", err)
	}
	password, err := rsa.DecryptPKCS1v15(nil, privateKey, b64EncryptedPassword[0:n])
	if err != nil {
		return "", fmt.Errorf("Failed to decrypt password: %s", err)
	}

	return string(password), nil
}
