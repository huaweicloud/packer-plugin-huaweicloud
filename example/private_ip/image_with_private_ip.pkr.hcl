packer {
  required_plugins {
    huaweicloud = {
      version = ">= 1.0.0"
      source  = "github.com/huaweicloud/huaweicloud"
    }
  }
}

source "huaweicloud-ecs" "example" {
  access_key = var.access_key
  secret_key = var.secret_key
  region     = var.region
  flavor     = "c6.large.2"
  image_name = "Ubuntu-2204-image-powered-by-Packer"
  image_tags = {
    builder = "packer"
    os      = "Ubuntu-22.04-server"
  }

  # there are 3 ways to fetch the source image that will be used to launch a new server.
  # 1. `source_image` --- The ID of the base image to use;
  # 2. `source_image_name` --- The name of the base image to use;
  # 3. `source_image_filter` --- Filter the base image by name, owner, visibility or tag;
  source_image_name = "Ubuntu 22.04 server 64bit"

  # if `associate_public_ip_address` is set to 'false', the following fields will be not allowed to use:
  # 1. `eip_type`;
  # 2. `eip_bandwidth_size`;
  # 3. `floating_ip`;
  # 4. `reuse_ips`;
  associate_public_ip_address = "false"
  vpc_id                      = var.vpc_id
  subnets                     = [var.subnet_id]
  security_groups             = [var.security_group_id]
  ssh_username                = "root"
}

build {
  sources = [
    "source.huaweicloud-ecs.example",
  ]

  provisioner "shell" {
    inline = ["apt-get update -y"]
  }

  post-processor "manifest" {
    strip_path = true
    output     = "packer-manifest.json"
  }
}
