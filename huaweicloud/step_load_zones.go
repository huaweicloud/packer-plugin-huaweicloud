package huaweicloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
	"github.com/huaweicloud/golangsdk"
)

type StepLoadAZ struct {
}

func (s *StepLoadAZ) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)

	client, err := config.computeV2Client()
	if err != nil {
		err = fmt.Errorf("Error initializing compute client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Loading available zones"))
	zones, err := listZones(client)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Message(fmt.Sprintf("Available zones: %s", strings.Join(zones, " ")))
	state.Put("azs", zones)
	return multistep.ActionContinue
}

func (s *StepLoadAZ) Cleanup(state multistep.StateBag) {
}

func listZones(client *golangsdk.ServiceClient) ([]string, error) {
	url := client.ServiceURL("os-availability-zone")
	r := golangsdk.Result{}
	_, r.Err = client.Get(url, &r.Body, &golangsdk.RequestOpts{
		MoreHeaders: map[string]string{"Content-Type": "application/json"}})
	if r.Err != nil {
		return nil, fmt.Errorf("Error getting zones, err=%s", r.Err)
	}

	type ZoneState struct {
		Available bool `json:"available"`
	}
	type ZoneInfo struct {
		ZoneName string    `json:"zoneName"`
		State    ZoneState `json:"zoneState"`
	}
	var body struct {
		ZoneInfos []ZoneInfo `json:"availabilityZoneInfo"`
	}
	err := r.ExtractInto(&body)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(body.ZoneInfos))
	for _, zoneInfo := range body.ZoneInfos {
		if zoneInfo.State.Available {
			result = append(result, zoneInfo.ZoneName)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("No available zones")
	}
	return result, nil
}
