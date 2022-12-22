# CHANGELOG

## 1.0.0 (December 21, 2022)

* Support `enterprise_project_id` option [GH-57]
* Support `data_disks` block and full-ECS image [GH-58]
* Support `security_token` option [GH-61]
* Support `wait_image_ready_timeout` option [GH-66]
* Add default DNS when creating subnet [GH-63]
* the following options are not supported:
  + *use_blockstorage_volume* and *force_delete* [GH-44]
  + *networks* and *ports*  [GH-45]
  + *image_min_disk* [GH-47]
  + *volume_name* [GH-58]

## 0.4.0 (April 26, 2021)

* support `packer init` command (#34)
* Upgrade plugin to use the new multi-component RPC server (#34)
* Update docs (#35)

## 0.3.0 (April 25, 2021)

* Upgrade **packer-plugin-sdk** to use version 0.2 to support Packer v1.7.0 and later (#32)

## 0.2.2 (March 29, 2021)

* Add new step add_image_members (#29)

## 0.2.1 (March 18, 2021)

* Update bandwidth charge_mode to traffic (#28)

## 0.2.0 (January 26, 2021)

* Support to log the golangsdk by HW_DEBUG env (#18)
* Use servers package to create image (#19)
* Use ims package to create image and support image_tags (#22)
* Merge volume_availability_zone into availability_zone (#16)
* Cleanup unused parameters (#15, #17, #20)

## 0.1.0 (January 25, 2021)

* First release in GitHub
