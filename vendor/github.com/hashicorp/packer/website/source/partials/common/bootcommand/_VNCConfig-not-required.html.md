<!-- Code generated from the comments of the VNCConfig struct in common/bootcommand/config.go; DO NOT EDIT MANUALLY -->

-   `disable_vnc` (bool) - Whether to create a VNC connection or not. A boot_command cannot be used
    when this is true. Defaults to false.
    
-   `boot_key_interval` (duration string | ex: "1h5m2s") - Time in ms to wait between each key press
    