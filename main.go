package main

import (
	"github.com/hashicorp/packer/packer/plugin"
	"github.com/zengchen1024/packer-builder-huaweicloud-ecs/huaweicloud"
)

func main() {
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}

	server.RegisterBuilder(new(huaweicloud.Builder))
	server.Serve()
}
