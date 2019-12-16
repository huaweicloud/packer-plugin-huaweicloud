---
description: |
    The huaweicloud-ecs Packer builder is able to create new images for use with
    Huawei Cloud. The builder takes a source image, runs any provisioning necessary
    on the image after launching it, then snapshots it into a reusable image. This
    reusable image can then be used as the foundation of new servers that are
    launched within Huawei Cloud.
layout: docs
page_title: 'HuaweiCloud-ECS - Builders'
sidebar_current: 'docs-builders-huaweicloud-ecs'
---

# HuaweiCloud-ECS Builder

Type: `huaweicloud-ecs`

The `huaweicloud-ecs` Packer builder is able to create new images for use with
[HuaweiCloud](https://www.huaweicloud.com). The builder takes a source image,
runs any provisioning necessary on the image after launching it, then snapshots
it into a reusable image. This reusable image can then be used as the
foundation of new servers that are launched within Huawei Cloud.

The builder does *not* manage images. Once it creates an image, it is up to you
to use it or delete it.

## Configuration Reference

There are many configuration options available for the builder. They are
segmented below into two categories: required and optional parameters.

In addition to the options listed here, a
[communicator](https://www.packer.io/docs/templates/communicator.html) can be configured for this
builder.

### Required:

-   `image_name` (string) - The name of the generated image.

-   `identity_endpoint` (string) - The URL of the Huawei Cloud Identity service.

-   `username` (string) - The username used to connect to the Huawei Cloud.

-   `password` (string) - The password used to connect to the Huawei Cloud.
    
-   `tenant_name` (string) - The project name to build resources in Huawei Cloud.

-   `domain_name` (string) - The Domain name to build resources in Huawei Cloud.

-   `insecure` (bool) - Whether or not the connection to Huawei Cloud can be done over an insecure connection. By default this is false.

-   `region` (string) - The region name to build resources in Huawei Cloud.

-   `source_image` (string) - The ID to the base image to use. This is the image that will be used to launch a new server and provision it. Unless you specify completely custom SSH settings, the source image must have cloud-init installed so that the keypair gets assigned properly.

-   `flavor` (string) - The name of desired flavor to create instance.

-   `vpc_id` (string) - The vpc id to attach instance.

-   `subnets` ([]string) - A list of subnets by UUID to attach instance.

-   `security_groups` ([]string) - A list of security groups by name to attach instance.
    
### Optional:

-   `eip_type` (string) - The type of eip. See the api doc to get the value..

-   `eip_bandwidth_size` (int) - The size of eip bandwidth.

-   `volume_size` (int) - The size of the system volume in GB. If this isn't specified, it is calculated from the source image bytes size.

## Communicator Configuration

### Optional:

-   `ssh_username` (string) - The username to connect to SSH with. Required if using SSH.

-   `ssh_ip_version` (string) - The IP version used for SSH connections, valid values are 4 and 6.

## Basic Example

Here is a basic builder example.

``` json
{
    "builders": [
        {
            "type": "huaweicloud-ecs",
            "image_name": "{{ image_name }}",
            "identity_endpoint": "https://iam.myhwclouds.com:443/v3",
            "username": "{{ username }}",
            "password": "{{ password }}",
            "tenant_name": "cn-north-1",
            "domain_name": "{{ domain_name }}",
            "insecure": "true",
            "region": "cn-north-1",
            "source_image": "{{ source_image }}",
            "flavor": "s3.medium.2",
	    "vpc_id": "{{ vpc_id }}",
            "subnets": [
                "{{ subnet }}"
            ],
            "security_groups": [
              "{{ security_group }}"
            ],
	    "eip_type": "5_bgp",
	    "eip_bandwidth_size": 2,
            "ssh_username": "root",
            "ssh_ip_version": "4",
        }
    ],

    "provisioners": [
        {
            "type": "shell",
            "inline": [
  	        "echo \"start install nginx, sleep 20s first\"",
       	        "sleep 20",
		"echo \"run install\"",
	    	"yum -y install nginx",
		"echo \"enable nginx\"",
		"systemctl enable nginx.service",
		"echo \"install nginx done\""
	    ]
	}
    ]
}
```
