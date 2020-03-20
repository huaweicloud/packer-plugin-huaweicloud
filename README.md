# Packer Builder for Huawei Cloud ECS

This is a [HashiCorp Packer](https://www.packer.io/) plugin for creating [Huawei Cloud ECS](https://www.huaweicloud.com/) image.

## Requirements
* [Packer 1.5.4](https://www.packer.io/intro/getting-started/install.html)
* [Go 1.13+](https://golang.org/doc/install)

## Build & Installation

### Install from source:

Clone repository to `$GOPATH/src/github.com/huaweicloud/packer-builder-huaweicloud-ecs`

```sh
$ mkdir -p $GOPATH/src/github.com/huaweicloud; cd $GOPATH/src/github.com/huaweicloud
$ git clone git@github.com:huaweicloud/packer-builder-huaweicloud-ecs.git
```

Enter the provider directory and build the provider

```sh
$ cd $GOPATH/src/github.com/huaweicloud/packer-builder-huaweicloud-ecs
$ make build
```

Link the build to Packer

```sh
$ln -s $GOPATH/bin/packer-builder-huaweicloud-ecs ~/.packer.d/plugins/packer-builder-huaweicloud-ecs
```

### Install from release:

* Download binaries from the [releases page](https://github.com/huaweicloud/packer-builder-huaweicloud-ecs/releases).
* [Install](https://www.packer.io/docs/extending/plugins.html#installing-plugins) the plugin, or simply put it into the same directory with JSON templates.
* Move the downloaded binary to `~/.packer.d/plugins/`

## Using the plugin
See the Huawei Cloud ECS Provider [documentation](website/source/docs/builders/huaweicloud-ecs.html.md) to get started.
