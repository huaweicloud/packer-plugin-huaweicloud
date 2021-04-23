package main

import (
	"github.com/hashicorp/packer-plugin-sdk/plugin"
	ecsbuilder "github.com/huaweicloud/packer-builder-huaweicloud-ecs/builder/ecs"
)

func main() {
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}

	server.RegisterBuilder(new(ecsbuilder.Builder))
	server.Serve()
}
