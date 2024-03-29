package huaweicloudimport

import (
	"fmt"
	"os"
	"testing"

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

func TestUploadFileToObject(t *testing.T) {
	region := os.Getenv("HW_REGION_NAME")
	ak := os.Getenv("HW_ACCESS_KEY")
	sk := os.Getenv("HW_SECRET_KEY")

	if region == "" || ak == "" || sk == "" {
		t.Skip("Skipping test because it requires some environment variables")
	}

	obsEndpoint := fmt.Sprintf("https://obs.%s.myhuaweicloud.com", region)
	envProxyConfigure := obs.WithProxyFromEnv(true)

	client, err := obs.New(ak, sk, obsEndpoint, obs.WithSignature("OBS"), envProxyConfigure)
	if err != nil {
		t.Fatalf("failed to create OBS client: %s", err)
	}

	err = uploadFileToObject(client, "image-test", "packer-import-test.raw", "./source-image-test.raw")
	if err != nil {
		t.Fatalf("failed to upload file: %s", err)
	}
}
