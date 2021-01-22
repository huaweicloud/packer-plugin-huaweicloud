package main

import (
	"github.com/hashicorp/packer-plugin-sdk/plugin"
	"github.com/huaweicloud/packer-builder-huaweicloud-ecs/huaweicloud"
)

func main() {
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}

	server.RegisterBuilder(new(huaweicloud.Builder))
	server.Serve()
}
