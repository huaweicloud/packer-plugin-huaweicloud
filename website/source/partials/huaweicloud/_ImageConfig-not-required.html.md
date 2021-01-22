<!-- Code generated from the comments of the ImageConfig struct in huaweicloud/image_config.go; DO NOT EDIT MANUALLY -->

-   `image_description` (string) - Specifies the image description.
    
-   `image_tags` (map[string]string) - The tags of the image in key/pair format.

-   `image_members` ([]string) - List of members to add to the image after creation. An image member is
    usually a project (also called the "tenant") with whom the image is
    shared.
    
-   `image_min_disk` (int) - Minimum disk size needed to boot image, in gigabytes.
    