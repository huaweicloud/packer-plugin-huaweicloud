<!-- Code generated from the comments of the ImageFilter struct in huaweicloud/run_config.go; DO NOT EDIT MANUALLY -->

- `filters` (ImageFilterOptions) - filters used to select a source_image. NOTE: This will fail unless
  exactly one image is returned, or most_recent is set to true.

- `most_recent` (bool) - Selects the newest created image when true. This is most useful for
  selecting a daily distro build.
