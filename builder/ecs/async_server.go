package ecs

import (
	"fmt"
	"log"
	"net/http"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	ecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	evs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/evs/v2"
	evsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/evs/v2/model"
)

// serverStateRefreshFunc returns a StateRefreshFunc that is used to watch an ECS server.
func serverStateRefreshFunc(client *ecs.EcsClient, serverID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		request := &ecsmodel.ShowServerRequest{
			ServerId: serverID,
		}

		response, err := client.ShowServer(request)
		if err != nil {
			var statusCode int
			if responseErr, ok := err.(*sdkerr.ServiceResponseError); ok {
				statusCode = responseErr.StatusCode
			} else {
				return nil, "ERROR", err
			}

			if statusCode == http.StatusNotFound {
				log.Printf("[INFO] 404 on ServerStateRefresh, returning DELETED")
				return nil, "DELETED", nil
			}

			log.Printf("[ERROR] Error on ServerStateRefresh: %s", err)
			return nil, "", err
		}

		serverNew := response.Server
		return serverNew, serverNew.Status, nil
	}
}

// serverJobStateRefreshFunc returns a StateRefreshFunc that is used to watch an ECS job.
func serverJobStateRefreshFunc(client *ecs.EcsClient, jobID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		request := &ecsmodel.ShowJobRequest{
			JobId: jobID,
		}

		response, err := client.ShowJob(request)
		if err != nil {
			log.Printf("[ERROR] Error on ServerJobStateRefresh: %s", err)
			return nil, "", err
		}

		status := response.Status.Value()
		return response, status, nil
	}
}

// volumeJobStateRefreshFunc returns a StateRefreshFunc that is used to watch an EVS job.
func volumeJobStateRefreshFunc(client *evs.EvsClient, jobID string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		request := &evsmodel.ShowJobRequest{
			JobId: jobID,
		}
		response, err := client.ShowJob(request)
		if err != nil {
			log.Printf("[ERROR] Error on VolumeJobStateRefresh: %s", err)
			return nil, "", err
		}

		status := response.Status.Value()
		if status == "SUCCESS" {
			return response, status, nil
		}
		if status == "FAIL" {
			return response, status, fmt.Errorf("the EVS job (%s) status is FAIL: %s",
				jobID, *response.FailReason)
		}
		return response, "PENDING", nil
	}
}
