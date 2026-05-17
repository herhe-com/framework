package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestApplicationIsSetUsesUnderlyingConfig(t *testing.T) {
	app := &Application{
		vip: viper.New(),
	}

	app.Set("app.name", "framework")

	if !app.IsSet("app.name") {
		t.Fatal("expected configured key to be set")
	}

	if app.IsSet("app.missing") {
		t.Fatal("expected missing key to be unset")
	}
}
