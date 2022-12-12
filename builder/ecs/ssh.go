package ecs

import (
	"errors"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/huaweicloud/golangsdk/openstack/compute/v2/servers"
	"github.com/huaweicloud/golangsdk/openstack/networking/v2/extensions/layer3/floatingips"
)

// CommHost looks up the host for the communicator.
func CommHost(host string) func(multistep.StateBag) (string, error) {
	return func(state multistep.StateBag) (string, error) {
		if host != "" {
			log.Printf("Using ssh_host value: %s", host)
			return host, nil
		}

		// if we have a floating IP, use that
		ip := state.Get("access_ip").(*floatingips.FloatingIP)
		if ip != nil && ip.FloatingIP != "" {
			log.Printf("[DEBUG] Using floating IP %s to connect", ip.FloatingIP)
			return ip.FloatingIP, nil
		}

		// try to get it from the requested interface
		if addr := getSshAddrFromPool(state); addr != "" {
			log.Printf("[DEBUG] Using IP address %s to connect", addr)
			return addr, nil
		}

		return "", errors.New("couldn't determine IP address for server")
	}
}

func getSshAddrFromPool(state multistep.StateBag) string {
	s := state.Get("server").(*servers.Server)
	var addr string

	// Get all the addresses associated with this server.
	for _, networkAddresses := range s.Addresses {
		elements, ok := networkAddresses.([]interface{})
		if !ok {
			log.Printf(
				"[ERROR] Unknown return type for address field: %#v",
				networkAddresses)
			continue
		}

		for _, element := range elements {
			nic := element.(map[string]interface{})
			if v, ok := nic["addr"]; ok {
				addr = v.(string)
			}

			if addr != "" {
				log.Printf("[DEBUG] Detected address: %s", addr)
				return addr
			}
		}
	}

	return ""
}
