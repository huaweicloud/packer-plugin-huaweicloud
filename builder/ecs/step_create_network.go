package ecs

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/random"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
	vpc "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpc/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpc/v2/model"
)

// refer to: https://support.huaweicloud.com/intl/en-us/dns_faq/dns_faq_002.html
var privateDNSList = map[string][]string{
	"cn-north-1":     {"100.125.1.250", "100.125.21.250"},  // Beijing-1
	"cn-north-4":     {"100.125.1.250", "100.125.129.250"}, // Beijing-4
	"cn-north-9":     {"100.125.1.250", "100.125.107.250"}, // Ulanqab
	"cn-east-2":      {"100.125.17.29", "100.125.135.29"},  // Shanghai-2
	"cn-east-3":      {"100.125.1.250", "100.125.64.250"},  // Shanghai-1
	"cn-south-1":     {"100.125.1.250", "100.125.136.29"},  // Guangzhou
	"cn-south-4":     {"100.125.0.167"},                    // Guangzhou-InvitationOnly
	"cn-southwest-2": {"100.125.1.250", "100.125.129.250"}, // Guiyang-1
	"ap-southeast-1": {"100.125.1.250", "100.125.3.250"},   // Hong Kong
	"ap-southeast-2": {"100.125.1.250", "100.125.1.251"},   // Bangkok
	"ap-southeast-3": {"100.125.1.250", "100.125.128.250"}, // Singapore
	"af-south-1":     {"100.125.1.250", "100.125.1.14"},    // Johannesburg
	"tr-west-1":      {"100.125.2.250", "100.125.2.251"},   // Turkey Istanbul
	"sa-brazil-1":    {"100.125.1.22", "100.125.1.90"},     // LA-Sao Paulo-1
	"na-mexico-1":    {"100.125.1.22", "100.125.1.90"},     // LA-Mexico City-1
	"la-north-2":     {"100.125.1.250", "100.125.1.242"},   // LA-Mexico City-2
	"la-south-2":     {"100.125.1.250", "100.125.0.250"},   // LA-Santiago
	"sa-chile-1":     {"100.125.1.250", "100.125.0.250"},   // LA-Santiago2
}

type StepCreateNetwork struct {
	VpcID          string
	Subnets        []string
	SecurityGroups []string
	doCleanup      bool
}

func (s *StepCreateNetwork) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	region := config.Region
	vpcClient, err := config.HcVpcClient(region)
	if err != nil {
		err = fmt.Errorf("Error initializing VPC client: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	if s.VpcID != "" {
		if len(s.Subnets) == 0 {
			err = fmt.Errorf("subnets must be specified with vpc_id")
			state.Put("error", err)
			return multistep.ActionHalt
		}

		// check the vpc
		request := &model.ShowVpcRequest{
			VpcId: s.VpcID,
		}

		_, err := vpcClient.ShowVpc(request)
		if err != nil {
			err := fmt.Errorf("Error loading VPC %s: %s", s.VpcID, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		state.Put("vpc_id", s.VpcID)
		state.Put("subnets", s.Subnets)
	} else {
		if len(s.Subnets) > 0 {
			err = fmt.Errorf("subnets must be empty if the vpc_id was not specified")
			state.Put("error", err)
			return multistep.ActionHalt
		}

		ui.Say("Creating temporary VPC...")
		vpcID, err := s.createVPC(vpcClient, config)
		if err != nil {
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		s.VpcID = vpcID
		ui.Message(fmt.Sprintf("temporary VPC ID: %s", vpcID))
		state.Put("vpc_id", vpcID)

		ui.Say("Creating temporary subnet...")
		subnetID, err := s.createSubnet(vpcClient, vpcID, region)
		if err != nil {
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		subnets := []string{subnetID}
		s.Subnets = subnets
		ui.Message(fmt.Sprintf("temporary subnet ID: %s", subnets[0]))
		state.Put("subnets", subnets)
	}

	if len(s.SecurityGroups) == 0 {
		ui.Message(fmt.Sprintf("the [default] security groups will be used ..."))
	} else {
		ui.Message(fmt.Sprintf("the %v security groups will be used ...", s.SecurityGroups))
	}

	return multistep.ActionContinue
}

func (s *StepCreateNetwork) Cleanup(state multistep.StateBag) {
	if !s.doCleanup {
		return
	}

	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)

	region := config.Region
	vpcClient, err := config.HcVpcClient(region)
	if err != nil {
		ui.Error(fmt.Sprintf("Error initializing VPC client: %s", err))
		return
	}

	if len(s.Subnets) > 0 {
		subnetID := s.Subnets[0]
		ui.Say(fmt.Sprintf("Deleting temporary subnet: %s...", subnetID))
		// Wait for the subnet be DELETED
		stateConf := StateChangeConf{
			Pending:    []string{"ACTIVE"},
			Target:     []string{"DELETED"},
			Refresh:    waitForSubnetDelete(vpcClient, s.VpcID, subnetID),
			Timeout:    3 * time.Minute,
			Delay:      3 * time.Second,
			MinTimeout: 5 * time.Second,
			StateBag:   state,
		}

		if _, err := stateConf.WaitForState(); err != nil {
			ui.Error(fmt.Sprintf(
				"Error cleaning up subnet %s. Please delete it manually: %s", subnetID, err))
		}
	}

	ui.Say(fmt.Sprintf("Deleting temporary VPC: %s...", s.VpcID))
	// Wait for the VPC be DELETED
	stateConf := StateChangeConf{
		Pending:    []string{"ACTIVE"},
		Target:     []string{"DELETED"},
		Refresh:    waitForVpcDelete(vpcClient, s.VpcID),
		Timeout:    3 * time.Minute,
		Delay:      3 * time.Second,
		MinTimeout: 5 * time.Second,
		StateBag:   state,
	}

	if _, err := stateConf.WaitForState(); err != nil {
		ui.Error(fmt.Sprintf(
			"Error cleaning up VPC %s. Please delete it manually: %s", s.VpcID, err))
	}
}

func (s *StepCreateNetwork) createVPC(client *vpc.VpcClient, conf *Config) (string, error) {
	vpcName := fmt.Sprintf("vpc-packer-%s", random.AlphaNumLower(6))
	vpcCIDR := "172.16.0.0/16"

	createOpts := model.CreateVpcOption{
		Name: &vpcName,
		Cidr: &vpcCIDR,
	}

	if conf.EnterpriseProjectId != "" {
		createOpts.EnterpriseProjectId = &conf.EnterpriseProjectId
	}
	request := &model.CreateVpcRequest{
		Body: &model.CreateVpcRequestBody{
			Vpc: &createOpts,
		},
	}
	response, err := client.CreateVpc(request)
	if err != nil {
		err := fmt.Errorf("Error creating VPC: %s", err)
		return "", err
	}

	if response.Vpc == nil {
		return "", fmt.Errorf("failed to obtain the VPC response")
	}

	s.doCleanup = true
	vpcID := response.Vpc.Id

	// Wait for VPC to become available.
	stateConf := StateChangeConf{
		Pending:    []string{"CREATING"},
		Target:     []string{"OK"},
		Refresh:    getVpcStatus(client, vpcID),
		Timeout:    3 * time.Minute,
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	if _, stateErr := stateConf.WaitForState(); stateErr != nil {
		vpcName := fmt.Sprintf("vpc-packer-%s", random.AlphaNumLower(6))
		err := fmt.Errorf("Error waiting for VPC %s(%s): %s", vpcName, vpcID, stateErr)
		return "", err
	}

	return vpcID, nil
}

func buildDNSList(region string) []string {
	if dnsList, ok := privateDNSList[region]; ok {
		return dnsList
	}

	// return public DNS: 8.8.8.8(google-public-dns-a.google.com) and 114.114.114.114(China)
	return []string{"8.8.8.8", "114.114.114.114"}
}

func (s *StepCreateNetwork) createSubnet(client *vpc.VpcClient, vpcID, region string) (string, error) {
	subnetName := fmt.Sprintf("subnet-packer-%s", random.AlphaNumLower(6))
	dnsList := buildDNSList(region)

	subnetOpts := model.CreateSubnetOption{
		VpcId:     vpcID,
		Name:      subnetName,
		Cidr:      "172.16.0.0/24",
		GatewayIp: "172.16.0.1",
		DnsList:   &dnsList,
	}

	subnetRequest := &model.CreateSubnetRequest{
		Body: &model.CreateSubnetRequestBody{
			Subnet: &subnetOpts,
		},
	}
	response, err := client.CreateSubnet(subnetRequest)
	if err != nil {
		err := fmt.Errorf("Error creating subnet: %s", err)
		return "", err
	}

	if response.Subnet == nil {
		return "", fmt.Errorf("failed to obtain the subnet response")
	}

	s.doCleanup = true
	subnetID := response.Subnet.Id

	// Wait for subnet to become available.
	stateConf := StateChangeConf{
		Pending:    []string{"UNKNOWN"},
		Target:     []string{"ACTIVE"},
		Refresh:    getSubnetStatus(client, subnetID),
		Timeout:    3 * time.Minute,
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	if _, stateErr := stateConf.WaitForState(); stateErr != nil {
		err := fmt.Errorf("Error waiting for subnet %s(%s): %s", subnetName, subnetID, stateErr)
		return "", err
	}

	return subnetID, nil
}

func getVpcStatus(client *vpc.VpcClient, vpcID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		request := &model.ShowVpcRequest{
			VpcId: vpcID,
		}

		response, err := client.ShowVpc(request)
		if err != nil {
			return nil, "", err
		}

		if response.Vpc == nil {
			return nil, "", fmt.Errorf("failed to obtain the VPC details")
		}

		status := response.Vpc.Status.Value()
		return response.Vpc, status, nil
	}
}

func getSubnetStatus(client *vpc.VpcClient, subnetID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		request := &model.ShowSubnetRequest{
			SubnetId: subnetID,
		}

		response, err := client.ShowSubnet(request)
		if err != nil {
			return nil, "", err
		}

		if response.Subnet == nil {
			return nil, "", fmt.Errorf("failed to obtain the subnet details")
		}

		status := response.Subnet.Status.Value()
		return response.Subnet, status, nil
	}
}

func waitForVpcDelete(client *vpc.VpcClient, vpcID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		request := &model.DeleteVpcRequest{
			VpcId: vpcID,
		}

		// the API response will be nil when got an error, but the wait do allow return with nil
		response := model.DeleteSubnetResponse{}
		if _, err := client.DeleteVpc(request); err != nil {
			var statusCode int
			if responseErr, ok := err.(*sdkerr.ServiceResponseError); ok {
				statusCode = responseErr.StatusCode
			} else {
				return response, "ERROR", err
			}

			switch statusCode {
			case http.StatusNotFound:
				log.Printf("[INFO] successfully delete VPC %s", vpcID)
				return response, "DELETED", nil
			case http.StatusConflict:
				log.Printf("[INFO] the VPC %s is still active", vpcID)
				return response, "ACTIVE", nil
			default:
				return response, "ACTIVE", err
			}
		}

		return response, "DELETED", nil
	}
}

func waitForSubnetDelete(client *vpc.VpcClient, vpcID, subnetID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		request := &model.DeleteSubnetRequest{
			VpcId:    vpcID,
			SubnetId: subnetID,
		}

		// the API response will be nil when got an error, but the wait do allow return with nil
		response := model.DeleteSubnetResponse{}
		if _, err := client.DeleteSubnet(request); err != nil {
			var statusCode int
			if responseErr, ok := err.(*sdkerr.ServiceResponseError); ok {
				statusCode = responseErr.StatusCode
			} else {
				return response, "ERROR", err
			}

			switch statusCode {
			case http.StatusNotFound:
				log.Printf("[INFO] successfully delete subnet %s", subnetID)
				return response, "DELETED", nil
			case http.StatusConflict:
				log.Printf("[INFO] the subnet %s is still active", subnetID)
				return response, "ACTIVE", nil
			default:
				return response, "ACTIVE", err
			}
		}

		return response, "DELETED", nil
	}
}
