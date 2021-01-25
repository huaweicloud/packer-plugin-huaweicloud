package huaweicloud

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/huaweicloud/golangsdk"
	"github.com/huaweicloud/golangsdk/openstack/compute/v2/servers"
)

// ServerStateChangeConf is the configuration struct used for `WaitForState`.
type ServerStateChangeConf struct {
	Pending   []string
	Refresh   StateRefreshFunc
	StepState multistep.StateBag
	Target    []string
}

// ServerStateRefreshFunc returns a StateRefreshFunc that is used to watch
// an HuaweiCloud server.
func ServerStateRefreshFunc(
	client *golangsdk.ServiceClient, serverID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		serverNew, err := servers.Get(client, serverID).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				log.Printf("[INFO] 404 on ServerStateRefresh, returning DELETED")
				return nil, "DELETED", nil
			}
			log.Printf("[ERROR] Error on ServerStateRefresh: %s", err)
			return nil, "", err
		}

		return serverNew, serverNew.Status, nil
	}
}

// WaitForState watches an object and waits for it to achieve a certain
// state.
func (conf *ServerStateChangeConf) WaitForState() (i interface{}, err error) {
	log.Printf("Waiting for state to become: %s", conf.Target)

	for {
		var currentState string
		i, currentState, err = conf.Refresh()
		if err != nil {
			return
		}

		for _, t := range conf.Target {
			if currentState == t {
				return
			}
		}

		if conf.StepState != nil {
			if _, ok := conf.StepState.GetOk(multistep.StateCancelled); ok {
				return nil, errors.New("interrupted")
			}
		}

		found := false
		for _, allowed := range conf.Pending {
			if currentState == allowed {
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("unexpected state '%s', wanted target '%s'", currentState, conf.Target)
		}

		log.Printf("Waiting for state to become: %s, currently: %s", conf.Target, currentState)
		time.Sleep(2 * time.Second)
	}
}
