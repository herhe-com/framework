package config

import "github.com/herhe-com/framework/facades"

// DefaultDisk returns the configured default filesystem disk name.
func DefaultDisk() string {
	return facades.Config().GetString("filesystem.default", "default")
}
