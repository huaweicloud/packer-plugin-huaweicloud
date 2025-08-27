# CHANGELOG

## 1.2.2 (August 24, 2025)

* Fix the version inconsistency issue after build.

## 1.2.1 (July 18, 2024)

* Fix the issue encountered in manual installation [#111](https://github.com/huaweicloud/packer-plugin-huaweicloud/issues/111).

## 1.2.0 (May 23, 2024)

# Support using private IP to login and install software.

## 1.1.0 (March 29, 2024)

* Support both plaint text or encoded with base64 format for `user_data` [GH-100]
* Support **post processor**: `huaweicloud-import` [GH-101]

## 1.0.4 (January 13, 2024)

* Fix an issue when waiting an EVS volume to become available [GH-98]

## 1.0.3 (July 7, 2023)

* Support filtering base images by tag [GH-78]
* Support TR-Istanbul region for private DNS and add default public DNS for other regions [GH-83]
* Support spot price mode when creating ECS [GH-84]
* Support encryption for system disk and data disks [GH-87]
* Support eu-west-101 region [GH-88]

## 1.0.2 (January 20, 2023)

* Support creating data volumes with `volume_id`, `snapshot_id` and `data_image_id` [GH-71]

## 1.0.1 (December 30, 2022)

* Support `image_type` option and support *data-disk* type image [GH-69]
* Support *system-data* type image [GH-70]

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
