//go:generate packer-sdc struct-markdown

package ecs

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"

	"github.com/huaweicloud/golangsdk"
	"github.com/huaweicloud/golangsdk/openstack"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/httphandler"

	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	eip "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2"
	ims "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ims/v2"
	vpc "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpc/v2"
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
	client, err := openstack.NewClient(c.IdentityEndpoint)
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

	client.HTTPClient = http.Client{
		Transport: &LogRoundTripper{
			Rt:    transport,
			Debug: logEnabled(),
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
	c.ProjectID = client.ProjectID
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

	return openstack.Authenticate(client, ao)
}

// NewHcClient is the common client using huaweicloud-sdk-go-v3 package
func NewHcClient(c *AccessConfig, region, product string) (*core.HcHttpClient, error) {
	endpoint := GetServiceEndpoint(product, region)
	if endpoint == "" {
		return nil, fmt.Errorf("failed to get the endpoint of %q service in region %s", product, region)
	}

	builder := core.NewHcHttpClientBuilder().WithEndpoint(endpoint).WithHttpConfig(buildHTTPConfig(c))

	credentials := basic.Credentials{
		AK:          c.AccessKey,
		SK:          c.SecretKey,
		ProjectId:   c.ProjectID,
		IamEndpoint: c.IdentityEndpoint,
	}
	builder.WithCredential(&credentials)

	return builder.Build(), nil
}

func buildHTTPConfig(c *AccessConfig) *config.HttpConfig {
	httpConfig := config.DefaultHttpConfig()

	if c.Insecure {
		httpConfig = httpConfig.WithIgnoreSSLVerification(true)
	}

	if logEnabled() {
		httpHandler := httphandler.NewHttpHandler().
			AddRequestHandler(logRequestHandler).
			AddResponseHandler(logResponseHandler)
		httpConfig = httpConfig.WithHttpHandler(httpHandler)
	}

	if proxyURL := getProxyFromEnv(); proxyURL != "" {
		if parsed, err := url.Parse(proxyURL); err == nil {
			log.Printf("[DEBUG] using https proxy: %s://%s", parsed.Scheme, parsed.Host)

			httpProxy := config.Proxy{
				Schema:   parsed.Scheme,
				Host:     parsed.Host,
				Username: parsed.User.Username(),
			}
			if pwd, ok := parsed.User.Password(); ok {
				httpProxy.Password = pwd
			}

			httpConfig = httpConfig.WithProxy(&httpProxy)
		} else {
			log.Printf("[WARN] parsing https proxy failed: %s", err)
		}
	}

	return httpConfig
}

// HcImsClient is the IMS service client using huaweicloud-sdk-go-v3 package
func (c *AccessConfig) HcImsClient(region string) (*ims.ImsClient, error) {
	hcClient, err := NewHcClient(c, region, "ims")
	if err != nil {
		return nil, err
	}

	return ims.NewImsClient(hcClient), nil
}

// HcEcsClient is the ECS service client using huaweicloud-sdk-go-v3 package
func (c *AccessConfig) HcEcsClient(region string) (*ecs.EcsClient, error) {
	hcClient, err := NewHcClient(c, region, "ecs")
	if err != nil {
		return nil, err
	}

	return ecs.NewEcsClient(hcClient), nil
}

// HcVpcClient is the VPC service client using huaweicloud-sdk-go-v3 package
func (c *AccessConfig) HcVpcClient(region string) (*vpc.VpcClient, error) {
	hcClient, err := NewHcClient(c, region, "vpc")
	if err != nil {
		return nil, err
	}

	return vpc.NewVpcClient(hcClient), nil
}

// HcEipClient is the EIP service client using huaweicloud-sdk-go-v3 package
func (c *AccessConfig) HcEipClient(region string) (*eip.EipClient, error) {
	hcClient, err := NewHcClient(c, region, "vpc")
	if err != nil {
		return nil, err
	}

	return eip.NewEipClient(hcClient), nil
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
	return openstack.NewNetworkV1(c.hwClient, golangsdk.EndpointOpts{
		Region:       c.Region,
		Availability: c.getEndpointType(),
	})
}

func (c *AccessConfig) getEndpointType() golangsdk.Availability {
	return golangsdk.AvailabilityPublic
}

func getProxyFromEnv() string {
	var url string

	envNames := []string{"HTTPS_PROXY", "https_proxy"}
	for _, n := range envNames {
		if val := os.Getenv(n); val != "" {
			url = val
			break
		}
	}

	return url
}

func logEnabled() bool {
	debugEnv := os.Getenv("HW_DEBUG")
	return debugEnv != "" && debugEnv != "0"
}
