package config

import "github.com/herhe-com/framework/facades"

// DefaultName returns the configured default queue connection name.
func DefaultName() string {
	return facades.Config().GetString("queue.default", "default")
}
