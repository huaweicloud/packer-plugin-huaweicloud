# Packer Plugin for HuaweiCloud ECS

This is a [HashiCorp Packer](https://www.packer.io/) plugin for creating [Huawei Cloud ECS](https://www.huaweicloud.com/) image.

## Installation

### Using pre-built releases

#### Using the `packer init` command

Starting from version 1.7, Packer supports a new `packer init` command allowing
automatic installation of Packer plugins. Read the
[Packer documentation](https://www.packer.io/docs/commands/init) for more information.

To install this plugin, copy and paste this code into your Packer configuration .
Then, run [`packer init`](https://www.packer.io/docs/commands/init).

```hcl
packer {
  required_plugins {
    huaweicloud = {
      version = ">= 0.4.0"
      source  = "github.com/huaweicloud/huaweicloud"
    }
  }
}
```

#### Manual installation

You can find pre-built binary releases of the plugin [here](https://github.com/huaweicloud/packer-plugin-huaweicloud/releases).
Once you have downloaded the latest archive corresponding to your target OS,
uncompress it to retrieve the plugin binary file corresponding to your platform.
To install the plugin, please follow the Packer documentation on
[installing a plugin](https://www.packer.io/docs/extending/plugins/#installing-plugins).


### Install from source:

If you prefer to build the plugin from source, clone the GitHub repository
to `$GOPATH/src/github.com/huaweicloud/packer-plugin-huaweicloud`.

```sh
$ mkdir -p $GOPATH/src/github.com/huaweicloud; cd $GOPATH/src/github.com/huaweicloud
$ git clone git@github.com:huaweicloud/packer-plugin-huaweicloud.git
```

Then enter the plugin directory and run `make build` command to build the plugin.

```sh
$ cd $GOPATH/src/github.com/huaweicloud/packer-plugin-huaweicloud
$ make build
```

Upon successful compilation, a `packer-plugin-huaweicloud` plugin binary file
can be found in the directory. To install the compiled plugin, please follow the
official Packer documentation on [installing a plugin](https://www.packer.io/docs/extending/plugins/#installing-plugins).
