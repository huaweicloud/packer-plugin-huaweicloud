package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
	"github.com/hashicorp/packer-plugin-sdk/version"
	ecsbuilder "github.com/huaweicloud/packer-builder-huaweicloud/builder/ecs"
	huaweicloudimport "github.com/huaweicloud/packer-builder-huaweicloud/post-processor/huaweicloud-import"
)

var (
	// Version is the main version number that is being run at the moment.
	Version = "1.2.0"

	// VersionPrerelease is A pre-release marker for the Version. If this is ""
	// (empty string) then it means that it is a final release. Otherwise, this
	// is a pre-release such as "dev" (in development), "beta", "rc1", etc.
	VersionPrerelease = ""

	// PluginVersion is used by the plugin set to allow Packer to recognize
	// what version this plugin is.
	PluginVersion = version.InitializePluginVersion(strings.TrimLeft(strings.ToLower(Version), "v"), VersionPrerelease)
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("ecs", new(ecsbuilder.Builder))
	pps.RegisterPostProcessor("import", new(huaweicloudimport.PostProcessor))
	pps.SetVersion(PluginVersion)
	err := pps.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
