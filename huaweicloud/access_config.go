package huaweicloud

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/huaweicloud/golangsdk"
	"github.com/huaweicloud/golangsdk/openstack"
	huaweisdk "github.com/huaweicloud/golangsdk/openstack"
)

const (
	UserAgent      = "packer-builder-huaweicloud-ecs"
	DefaultAuthURL = "https://iam.myhuaweicloud.com:443/v3"
)

// AccessConfig is for common configuration related to HuaweiCloud access
type AccessConfig struct {
	// The access key of the HuaweiCloud to use.
	// If omitted, the HW_ACCESS_KEY environment variable is used.
	AccessKey string `mapstructure:"access_key" required:"true"`
	// The secret key of the HuaweiCloud to use.
	// If omitted, the HW_SECRET_KEY environment variable is used.
	SecretKey string `mapstructure:"secret_key" required:"true"`
	// Specifies the HuaweiCloud region in which to launch the server to create the image.
	// If omitted, the HW_REGION_NAME environment variable is used.
	Region string `mapstructure:"region" required:"true"`

	// The Name of the project to login with.
	// If omitted, the HW_PROJECT_NAME environment variable or Region is used.
	ProjectName string `mapstructure:"project_name" required:"false"`
	// The ID of the project to login with.
	// If omitted, the HW_PROJECT_ID environment variable is used.
	ProjectID string `mapstructure:"project_id" required:"false"`

	// The Identity authentication URL.
	// If omitted, the HW_AUTH_URL environment variable is used.
	// This is not required if you use HuaweiCloud.
	IdentityEndpoint string `mapstructure:"auth_url" required:"false"`
	// Trust self-signed SSL certificates.
	// By default this is false.
	Insecure bool `mapstructure:"insecure" required:"false"`

	hwClient *golangsdk.ProviderClient
}

func (c *AccessConfig) Prepare(ctx *interpolate.Context) []error {

	if c.AccessKey == "" {
		c.AccessKey = os.Getenv("HW_ACCESS_KEY")
	}
	if c.SecretKey == "" {
		c.SecretKey = os.Getenv("HW_SECRET_KEY")
	}
	if c.Region == "" {
		c.Region = os.Getenv("HW_REGION_NAME")
	}
	// access parameters validation
	if c.AccessKey == "" || c.SecretKey == "" || c.Region == "" {
		paraErr := fmt.Errorf("access_key, secret_key and region must be set")
		return []error{paraErr}
	}

	if c.ProjectID == "" {
		c.ProjectID = os.Getenv("HW_PROJECT_ID")
	}
	if c.ProjectName == "" {
		c.ProjectName = os.Getenv("HW_PROJECT_NAME")
	}
	// if neither "project_name" nor HW_PROJECT_NAME was specified, defaults to c.Region
	if c.ProjectName == "" {
		c.ProjectName = c.Region
	}

	if c.IdentityEndpoint == "" {
		c.IdentityEndpoint = os.Getenv("HW_AUTH_URL")
	}
	// if neither "auth_url" nor HW_AUTH_URL was specified, defaults to DefaultAuthURL
	if c.IdentityEndpoint == "" {
		c.IdentityEndpoint = DefaultAuthURL
	}

	// initialize the ProviderClient
	client, err := huaweisdk.NewClient(c.IdentityEndpoint)
	if err != nil {
		return []error{err}
	}

	// Set UserAgent
	client.UserAgent.Prepend(UserAgent)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.Insecure,
	}
	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}

	var enableLog bool
	debugEnv := os.Getenv("HW_DEBUG")
	if debugEnv != "" && debugEnv != "0" {
		enableLog = true
	}

	client.HTTPClient = http.Client{
		Transport: &LogRoundTripper{
			Rt:    transport,
			Debug: enableLog,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if client.AKSKAuthOptions.AccessKey != "" {
				golangsdk.ReSign(req, golangsdk.SignOptions{
					AccessKey: client.AKSKAuthOptions.AccessKey,
					SecretKey: client.AKSKAuthOptions.SecretKey,
				})
			}
			return nil
		},
	}

	err = buildClientByAKSK(c, client)
	if err != nil {
		return []error{err}
	}
	c.hwClient = client
	return nil
}

func buildClientByAKSK(c *AccessConfig, client *golangsdk.ProviderClient) error {
	ao := golangsdk.AKSKAuthOptions{
		AccessKey:        c.AccessKey,
		SecretKey:        c.SecretKey,
		ProjectName:      c.ProjectName,
		ProjectId:        c.ProjectID,
		IdentityEndpoint: c.IdentityEndpoint,
	}

	return huaweisdk.Authenticate(client, ao)
}

func (c *AccessConfig) computeV2Client() (*golangsdk.ServiceClient, error) {
	return openstack.NewComputeV2(c.hwClient, golangsdk.EndpointOpts{
		Region:       c.Region,
		Availability: c.getEndpointType(),
	})
}

func (c *AccessConfig) imageV2Client() (*golangsdk.ServiceClient, error) {
	return openstack.NewImageServiceV2(c.hwClient, golangsdk.EndpointOpts{
		Region:       c.Region,
		Availability: c.getEndpointType(),
	})
}

func (c *AccessConfig) blockStorageV3Client() (*golangsdk.ServiceClient, error) {
	return openstack.NewBlockStorageV3(c.hwClient, golangsdk.EndpointOpts{
		Region:       c.Region,
		Availability: c.getEndpointType(),
	})
}

func (c *AccessConfig) networkV2Client() (*golangsdk.ServiceClient, error) {
	return openstack.NewNetworkV2(c.hwClient, golangsdk.EndpointOpts{
		Region:       c.Region,
		Availability: c.getEndpointType(),
	})
}

func (c *AccessConfig) vpcClient() (*golangsdk.ServiceClient, error) {
	return huaweisdk.NewNetworkV1(c.hwClient, golangsdk.EndpointOpts{
		Region:       c.Region,
		Availability: c.getEndpointType(),
	})
}

func (c *AccessConfig) getEndpointType() golangsdk.Availability {
	return golangsdk.AvailabilityPublic
}
