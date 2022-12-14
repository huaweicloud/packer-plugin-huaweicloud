package ecs

import (
	"fmt"
	"net/url"
	"strings"
)

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
	"vpc": {
		Name: "vpc",
	},
	"eip": {
		Name: "vpc",
	},
}

// GetServiceEndpoint try to get the endpoint from customizing map
func GetServiceEndpoint(cloud, srv, region string) string {
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

func GetCloudFromAuth(auth string) (string, error) {
	var cloud string

	u, err := url.Parse(auth)
	if err != nil {
		return "", err
	}

	// the parsed Host in host:port format, get rid of the port
	hosts := strings.SplitN(u.Host, ":", 2)

	subhosts := strings.Split(hosts[0], ".")
	total := len(subhosts)
	if total == 3 {
		// without region: iam.myhuaweicloud.com
		cloud = strings.Join(subhosts[1:], ".")
	} else if total > 3 {
		// with region: iam.cn-north-1.myhuaweicloud.com
		// iam.eu-west-0.prod-cloud-ocb.orange-business.com
		cloud = strings.Join(subhosts[2:], ".")
	} else {
		return "", fmt.Errorf("the auth_url is invalid")
	}

	return cloud, nil
}
