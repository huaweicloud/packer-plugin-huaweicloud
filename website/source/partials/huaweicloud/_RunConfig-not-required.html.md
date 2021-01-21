<!-- Code generated from the comments of the RunConfig struct in huaweicloud/run_config.go; DO NOT EDIT MANUALLY -->

-   `ssh_interface` (string) - The type of interface to connect via SSH. Values useful for Rackspace
    are "public" or "private", and the default behavior is to connect via
    whichever is returned first from the OpenStack API.
    
-   `ssh_ip_version` (string) - The IP version to use for SSH connections, valid values are `4` and `6`.
    Useful on dual stacked instances where the default behavior is to
    connect via whichever IP address is returned first from the OpenStack
    API.
    
-   `availability_zone` (string) - The availability zone to launch the server in.
    If omitted, a random availability zone in the region will be used.
    
-   `floating_ip` (string) - A specific floating IP to assign to this instance.
    
-   `reuse_ips` (bool) - Whether or not to attempt to reuse existing unassigned floating ips in
    the project before allocating a new one. Note that it is not possible to
    safely do this concurrently, so if you are running multiple openstack
    builds concurrently, or if other processes are assigning and using
    floating IPs in the same openstack project while packer is running, you
    should not set this to true. Defaults to false.
    
-   `eip_type` (string) - The type of eip. See the api doc to get the value.
    
-   `eip_bandwidth_size` (int) - The size of eip bandwidth.
    
-   `security_groups` ([]string) - A list of security groups by name to add to this instance.
    
-   `networks` ([]string) - A list of networks by UUID to attach to this instance.
    
-   `ports` ([]string) - A list of ports by UUID to attach to this instance.
    
-   `vpc_id` (string) - A vpc id to attach to this instance.
    
-   `subnets` ([]string) - A list of subnets by UUID to attach to this instance.
    
-   `user_data` (string) - User data to apply when launching the instance. Note that you need to be
    careful about escaping characters due to the templates being JSON. It is
    often more convenient to use user_data_file, instead. Packer will not
    automatically wait for a user script to finish before shutting down the
    instance this must be handled in a provisioner.
    
-   `user_data_file` (string) - Path to a file that will be used for the user data when launching the
    instance.
    
-   `instance_name` (string) - Name that is applied to the server instance created by Packer. If this
    isn't specified, the default is same as image_name.
    
-   `instance_metadata` (map[string]string) - Metadata that is applied to the server instance created by Packer. Also
    called server properties in some documentation. The strings have a max
    size of 255 bytes each.
    
-   `force_delete` (bool) - Whether to force the OpenStack instance to be forcefully deleted. This
    is useful for environments that have reclaim / soft deletion enabled. By
    default this is false.
    
-   `config_drive` (bool) - Whether or not nova should use ConfigDrive for cloud-init metadata.
    
-   `use_blockstorage_volume` (bool) - Use Block Storage service volume for the instance root volume instead of
    Compute service local volume (default).
    
-   `volume_name` (string) - Name of the Block Storage service volume. If this isn't specified,
    random string will be used.
    
-   `volume_type` (string) - Type of the Block Storage service volume. If this isn't specified, the
    default enforced by your OpenStack cluster will be used.
    
-   `volume_size` (int) - Size of the Block Storage service volume in GB. If this isn't specified,
    it is set to source image min disk value (if set) or calculated from the
    source image bytes size. Note that in some cases this needs to be
    specified, if use_blockstorage_volume is true.
    
