package config

import "github.com/herhe-com/framework/facades"

// DefaultDisk returns the configured default filesystem disk name.
func DefaultDisk() string {
	return facades.Config().GetString("filesystem.default", "default")
}

// Driver returns the configured driver for a filesystem disk.
func Driver(disk, defaultValue string) string {
	if driver := facades.Config().GetString("filesystem.disks." + disk + ".driver"); driver != "" {
		return driver
	}

	return defaultValue
}
