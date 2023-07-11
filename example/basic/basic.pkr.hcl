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

  # `eip_type` and `eip_bandwidth_size` are used to create a temporary EIP to ssh the server.
  # if you want to reuse existing unassigned EIPs, please use `reuse_ips` or `floating_ip`.
  eip_type           = "5_bgp"
  eip_bandwidth_size = 5
  ssh_username       = "root"
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
