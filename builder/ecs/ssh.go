package ecs

import (
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

// CommHost looks up the host for the communicator.
func CommHost(host string) func(multistep.StateBag) (string, error) {
	return func(state multistep.StateBag) (string, error) {
		if host != "" {
			log.Printf("Using ssh_host value: %s", host)
			return host, nil
		}

		// if we have a floating IP, use that
		if rst, ok := state.GetOk("access_eip"); ok {
			publicIP := rst.(*PublicipIP)
			log.Printf("[DEBUG] Using floating IP %s to connect", publicIP.Address)
			return publicIP.Address, nil
		}

		// use the primary private IP
		privateIP := state.Get("access_private_ip").(string)
		log.Printf("[DEBUG] Using IP address %s to connect", privateIP)
		return privateIP, nil
	}
}
