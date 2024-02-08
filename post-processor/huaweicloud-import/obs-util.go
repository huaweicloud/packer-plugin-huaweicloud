package huaweicloudimport

import (
	"fmt"
	"log"

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
	ecsbuilder "github.com/huaweicloud/packer-builder-huaweicloud/builder/ecs"
)

func (p *PostProcessor) newOBSClient(region string) (*obs.ObsClient, error) {
	conf := p.config
	obsEndpoint := ecsbuilder.GetServiceEndpoint(conf.Cloud, "obs", region)
	envProxyConfigure := obs.WithProxyFromEnv(true)

	if conf.SecurityToken != "" {
		return obs.New(conf.AccessKey, conf.SecretKey, obsEndpoint,
			obs.WithSignature("OBS"), obs.WithSecurityToken(conf.SecurityToken), envProxyConfigure)
	}
	return obs.New(conf.AccessKey, conf.SecretKey, obsEndpoint, obs.WithSignature("OBS"), envProxyConfigure)
}

func queryBucket(client *obs.ObsClient, bucketName string) error {
	_, err := client.HeadBucket(bucketName)
	if err != nil {
		return fmt.Errorf("error on reading bucket %s: %s", bucketName, err)
	}

	return nil
}

func uploadFileToObject(client *obs.ObsClient, bucketName, keyName, sourceFile string) error {
	putInput := &obs.PutFileInput{}
	putInput.Bucket = bucketName
	putInput.Key = keyName
	putInput.SourceFile = sourceFile

	log.Printf("[DEBUG] uploading %s to OBS bucket %s, opts: %#v", keyName, bucketName, putInput)
	_, err := client.PutFile(putInput)
	if err != nil {
		return fmt.Errorf("failed to upload object to OBS bucket %s: %s", bucketName, err)
	}
	return nil
}

func deleteFile(client *obs.ObsClient, bucketName, keyName string) error {
	input := &obs.DeleteObjectInput{
		Bucket: bucketName,
		Key:    keyName,
	}

	log.Printf("[DEBUG] Object %s will be deleted with all versions", keyName)
	_, err := client.DeleteObject(input)
	if err != nil {
		return fmt.Errorf("error deleting object of OBS bucket %s: %s", bucketName, err)
	}

	return nil
}
