package ecs

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// StepLoadFlavor gets the FlavorRef from a Flavor.
type StepLoadFlavor struct {
	Flavor string
}

func (s *StepLoadFlavor) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)

	ui.Say(fmt.Sprintf("Loading flavor: %s", s.Flavor))
	log.Printf("[INFO] skip the verification for flavor %s as the method is missing in SDK", s.Flavor)

	// ui.Message(fmt.Sprintf("Verified flavor ID: %s", s.Flavor))
	state.Put("flavor_id", s.Flavor)
	return multistep.ActionContinue
}

func (s *StepLoadFlavor) Cleanup(state multistep.StateBag) {
}
