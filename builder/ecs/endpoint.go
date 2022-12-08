package ecs

import "fmt"

type ServiceCatalog struct {
	Name  string
	Scope string
	Admin bool
}

var serviceEndpoints = map[string]ServiceCatalog{
	"ecs": {
		Name: "ecs",
	},
	"ims": {
		Name: "ims",
	},
	"evs": {
		Name: "evs",
	},
	"vpc": {
		Name: "vpc",
	},
}

// GetServiceEndpoint try to get the endpoint from customizing map
func GetServiceEndpoint(srv, region string) string {
	var cloud string = "myhuaweicloud.com"
	// get the endpoint from build-in service catalog
	catalog, ok := serviceEndpoints[srv]
	if !ok {
		return ""
	}

	var ep string
	if catalog.Scope == "global" {
		ep = fmt.Sprintf("https://%s.%s/", catalog.Name, cloud)
	} else {
		ep = fmt.Sprintf("https://%s.%s.%s/", catalog.Name, region, cloud)
	}
	return ep
}
