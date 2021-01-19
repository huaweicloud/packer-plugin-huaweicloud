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

-  `access_key` (string) - The access key of the HuaweiCloud to use. If omitted, the *HW_ACCESS_KEY* environment variable is used.

-  `secret_key` (string) - The secret key of the HuaweiCloud to use. If omitted, the *HW_SECRET_KEY* environment variable is used.

-   `region` (string) - The region name to build resources in Huawei Cloud. If omitted, the *HW_REGION_NAME* environment variable is used.

-   `image_name` (string) - The name of the generated image.

-   `source_image` (string) - The ID to the base image to use. This is the image that will be used to launch a new server and provision it. Unless you specify completely custom SSH settings, the source image must have cloud-init installed so that the keypair gets assigned properly.

-   `flavor` (string) - The name of desired flavor to create instance.

-   `vpc_id` (string) - The vpc id to attach instance.

-   `subnets` ([]string) - A list of subnets by UUID to attach instance.

-   `security_groups` ([]string) - A list of security groups by name to attach instance.
    
### Optional:

-   `availability_zone` (string) - The availability zone to build resources in Huawei Cloud.

-   `eip_type` (string) - The type of eip. See the api doc to get the value..

-   `eip_bandwidth_size` (int) - The size of eip bandwidth.

-   `volume_size` (int) - The size of the system volume in GB. If this isn't specified, it is calculated from the source image bytes size.

-   `project_name` (string) - The project name to build resources in Huawei Cloud.
    If omitted, the *HW_PROJECT_NAME* environment variable or `region` is used.

-   `project_id` (string) - The project ID to build resources in Huawei Cloud.
    If omitted, the *HW_PROJECT_ID* environment variable is used.

-   `auth_url` (string) - The URL of the Huawei Cloud Identity service. If omitted, the *HW_AUTH_URL* environment variable is used.
    This is not required if you use Huawei Cloud.

-   `insecure` (bool) - Whether or not the connection to Huawei Cloud can be done over an insecure connection. By default this is false.

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
            "access_key": "{{ my-access-key }}",
            "secret_key": "{{ my-secret-key }}",
            "region": "cn-north-1",
            "image_name": "{{ image_name }}",
            "source_image": "{{ source_image }}",
            "flavor": "s6.large.2",
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
