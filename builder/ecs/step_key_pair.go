package ecs

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/tmp"
	"github.com/hashicorp/packer-plugin-sdk/uuid"
	"golang.org/x/crypto/ssh"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
)

type StepKeyPair struct {
	Debug        bool
	Comm         *communicator.Config
	DebugKeyPath string

	doCleanup bool
}

func (s *StepKeyPair) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)

	if s.Comm.SSHPrivateKeyFile != "" {
		ui.Say("Using existing SSH private key")
		privateKeyBytes, err := s.Comm.ReadSSHPrivateKeyFile()
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}

		key, err := ssh.ParsePrivateKey(privateKeyBytes)
		if err != nil {
			err = fmt.Errorf("Error parsing 'ssh_private_key_file': %s", err)
			ui.Error(err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}

		config := state.Get("config").(*Config)
		region := config.Region
		ecsClient, err := config.HcEcsClient(region)
		if err != nil {
			err = fmt.Errorf("Error initializing compute client: %s", err)
			state.Put("error", err)
			return multistep.ActionHalt
		}

		s.Comm.SSHTemporaryKeyPairName = fmt.Sprintf("packer_%s", uuid.TimeOrderedUUID())
		kpName := s.Comm.SSHTemporaryKeyPairName
		ui.Say(fmt.Sprintf("Creating temporary keypair using provided private key: %s...", kpName))

		strPubKey := string(ssh.MarshalAuthorizedKey(key.PublicKey()))
		keypairbody := &model.NovaCreateKeypairOption{
			Name:      kpName,
			PublicKey: &strPubKey,
		}
		request := &model.NovaCreateKeypairRequest{
			Body: &model.NovaCreateKeypairRequestBody{
				Keypair: keypairbody,
			},
		}

		response, err := ecsClient.NovaCreateKeypair(request)
		if err != nil {
			state.Put("error", fmt.Errorf("Error creating temporary keypair: %s", err))
			return multistep.ActionHalt
		}

		if response.Keypair == nil || response.Keypair.Fingerprint == "" {
			state.Put("error", fmt.Errorf("The temporary keypair returned was blank"))
			return multistep.ActionHalt
		}

		ui.Say(fmt.Sprintf("Created temporary keypair: %s", kpName))

		// we created a temporary key, so remember to clean it up
		s.doCleanup = true

		// Set some state data for use in future steps
		s.Comm.SSHKeyPairName = kpName
		s.Comm.SSHPrivateKey = privateKeyBytes
		s.Comm.SSHPublicKey = ssh.MarshalAuthorizedKey(key.PublicKey())

		return multistep.ActionContinue
	}

	if s.Comm.SSHAgentAuth && s.Comm.SSHKeyPairName == "" {
		ui.Say("Using SSH Agent with key pair in Source image")
		return multistep.ActionContinue
	}

	if s.Comm.SSHAgentAuth && s.Comm.SSHKeyPairName != "" {
		ui.Say(fmt.Sprintf("Using SSH Agent for existing key pair %s", s.Comm.SSHKeyPairName))
		s.Comm.SSHKeyPairName = ""
		return multistep.ActionContinue
	}

	if s.Comm.SSHTemporaryKeyPairName == "" {
		ui.Say("Not using temporary keypair")
		s.Comm.SSHKeyPairName = ""
		return multistep.ActionContinue
	}

	config := state.Get("config").(*Config)
	region := config.Region
	ecsClient, err := config.HcEcsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	kpName := s.Comm.SSHTemporaryKeyPairName
	ui.Say(fmt.Sprintf("Creating temporary keypair: %s...", kpName))

	keypairbody := &model.NovaCreateKeypairOption{
		Name: kpName,
	}
	request := &model.NovaCreateKeypairRequest{
		Body: &model.NovaCreateKeypairRequestBody{
			Keypair: keypairbody,
		},
	}

	response, err := ecsClient.NovaCreateKeypair(request)
	if err != nil {
		state.Put("error", fmt.Errorf("Error creating temporary keypair: %s", err))
		return multistep.ActionHalt
	}

	if response.Keypair == nil || response.Keypair.PrivateKey == "" {
		state.Put("error", fmt.Errorf("The temporary keypair returned was blank"))
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Created temporary keypair: %s", kpName))

	privateKey := string(berToDer([]byte(response.Keypair.PrivateKey), ui))

	// If we're in debug mode, output the private key to the working
	// directory.
	if s.Debug {
		ui.Message(fmt.Sprintf("Saving key for debug purposes: %s", s.DebugKeyPath))
		f, err := os.Create(s.DebugKeyPath)
		if err != nil {
			state.Put("error", fmt.Errorf("Error saving debug key: %s", err))
			return multistep.ActionHalt
		}
		defer f.Close()

		// Write the key out
		if _, err := f.Write([]byte(privateKey)); err != nil {
			state.Put("error", fmt.Errorf("Error saving debug key: %s", err))
			return multistep.ActionHalt
		}

		// Chmod it so that it is SSH ready
		if runtime.GOOS != "windows" {
			if err := f.Chmod(0600); err != nil {
				state.Put("error", fmt.Errorf("Error setting permissions of debug key: %s", err))
				return multistep.ActionHalt
			}
		}
	}

	// we created a temporary key, so remember to clean it up
	s.doCleanup = true

	// Set some state data for use in future steps
	s.Comm.SSHKeyPairName = kpName
	s.Comm.SSHPrivateKey = []byte(privateKey)

	return multistep.ActionContinue
}

// Work around for https://github.com/hashicorp/packer/issues/2526
func berToDer(ber []byte, ui packer.Ui) []byte {
	// Check if x/crypto/ssh can parse the key
	_, err := ssh.ParsePrivateKey(ber)
	if err == nil {
		return ber
	}
	// Can't parse the key, maybe it's BER encoded. Try to convert it with OpenSSL.
	log.Println("Couldn't parse SSH key, trying work around for [GH-2526].")

	openSslPath, err := exec.LookPath("openssl")
	if err != nil {
		log.Println("Couldn't find OpenSSL, aborting work around.")
		return ber
	}

	berKey, err := tmp.File("packer-ber-privatekey-")
	defer os.Remove(berKey.Name())
	if err != nil {
		return ber
	}
	ioutil.WriteFile(berKey.Name(), ber, os.ModeAppend)
	derKey, err := tmp.File("packer-der-privatekey-")
	defer os.Remove(derKey.Name())
	if err != nil {
		return ber
	}

	args := []string{"rsa", "-in", berKey.Name(), "-out", derKey.Name()}
	log.Printf("Executing: %s %v", openSslPath, args)
	if err := exec.Command(openSslPath, args...).Run(); err != nil {
		log.Printf("OpenSSL failed with error: %s", err)
		return ber
	}

	der, err := ioutil.ReadFile(derKey.Name())
	if err != nil {
		return ber
	}
	ui.Say("Successfully converted BER encoded SSH key to DER encoding.")
	return der
}

func (s *StepKeyPair) Cleanup(state multistep.StateBag) {
	if !s.doCleanup {
		return
	}

	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)

	kpName := s.Comm.SSHTemporaryKeyPairName
	region := config.Region
	ecsClient, err := config.HcEcsClient(region)
	if err != nil {
		ui.Error(fmt.Sprintf(
			"Error cleaning up keypair %s. Please delete the key manually: %s", kpName, err))
		return
	}

	ui.Say(fmt.Sprintf("Deleting temporary keypair: %s ...", kpName))
	request := &model.NovaDeleteKeypairRequest{
		KeypairName: kpName,
	}

	_, err = ecsClient.NovaDeleteKeypair(request)
	if err != nil {
		ui.Error(fmt.Sprintf(
			"Error cleaning up keypair %s. Please delete the key manually: %s", kpName, err))
	}
}
