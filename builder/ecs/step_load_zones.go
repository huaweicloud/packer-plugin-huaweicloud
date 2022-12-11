package ecs

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
)

type StepLoadAZ struct {
	AvailabilityZone string
}

func (s *StepLoadAZ) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)

	region := config.Region
	client, err := config.HcEcsClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Loading availability zones..."))
	zones, err := listZones(client)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	if s.AvailabilityZone != "" {
		isExist := false
		for _, az := range zones {
			if az == s.AvailabilityZone {
				isExist = true
				break
			}
		}
		if !isExist {
			err = fmt.Errorf("the specified availability_zone %s is not exist or available", s.AvailabilityZone)
			state.Put("error", err)
			return multistep.ActionHalt
		}
		ui.Message(fmt.Sprintf("the specified availability_zone %s is available", s.AvailabilityZone))
	} else {
		ui.Message(fmt.Sprintf("Availability zones: %s", strings.Join(zones, " ")))
		// select an rand availability zone
		randIndex := rand.Intn(len(zones))
		s.AvailabilityZone = zones[randIndex]
		ui.Message(fmt.Sprintf("Select %s as the availability zone", s.AvailabilityZone))
	}

	state.Put("availability_zone", s.AvailabilityZone)
	return multistep.ActionContinue
}

func (s *StepLoadAZ) Cleanup(state multistep.StateBag) {
}

func listZones(client *ecs.EcsClient) ([]string, error) {
	response, err := client.NovaListAvailabilityZones(nil)
	if err != nil {
		return nil, fmt.Errorf("Error getting zones, err=%s", err)
	}

	zoneInfos := *response.AvailabilityZoneInfo
	result := make([]string, 0, len(zoneInfos))
	for _, zone := range zoneInfos {
		if zone.ZoneState.Available {
			result = append(result, zone.ZoneName)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("No available zones")
	}
	return result, nil
}
