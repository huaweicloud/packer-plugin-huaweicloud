<!-- Code generated from the comments of the ImageConfig struct in huaweicloud/image_config.go; DO NOT EDIT MANUALLY -->

-   `metadata` (map[string]string) - Glance metadata that will be applied to the image.
    
-   `image_members` ([]string) - List of members to add to the image after creation. An image member is
    usually a project (also called the "tenant") with whom the image is
    shared.
    
-   `image_tags` ([]string) - List of tags to add to the image after creation.
    
-   `image_min_disk` (int) - Minimum disk size needed to boot image, in gigabytes.
    