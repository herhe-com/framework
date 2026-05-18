package foundation

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"

	contractconfig "github.com/herhe-com/framework/contracts/config"
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

type fakeConfig struct {
	values map[string]any
}

func (f fakeConfig) Env(key string, defaultValue ...any) any {
	return f.Get(key, defaultValue...)
}

func (f fakeConfig) Add(name string, configuration map[string]any) {}

func (f fakeConfig) Set(key string, configuration any) {}

func (f fakeConfig) Get(key string, defaultValue ...any) any {
	if value, ok := f.values[key]; ok {
		return value
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return nil
}

func (f fakeConfig) GetString(key string, defaultValue ...string) string {
	if value, ok := f.values[key]; ok {
		return fmt.Sprint(value)
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

func (f fakeConfig) GetStrings(key string, defaultValue ...[]string) []string {
	return nil
}

func (f fakeConfig) GetMaps(key string, defaultValue ...map[string]any) map[string]any {
	return nil
}

func (f fakeConfig) GetInt(key string, defaultValue ...int) int {
	return 0
}

func (f fakeConfig) GetInt64(key string, defaultValue ...int64) int64 {
	return 0
}

func (f fakeConfig) GetBool(key string, defaultValue ...bool) bool {
	return false
}

func (f fakeConfig) IsSet(key string) bool {
	_, ok := f.values[key]
	return ok
}

type failingRegisterProvider struct{}

func (failingRegisterProvider) Register() error {
	return errors.New("register failed")
}

func (failingRegisterProvider) Boot() error {
	return nil
}

type failingBootProvider struct{}

func (failingBootProvider) Register() error {
	return nil
}

func (failingBootProvider) Boot() error {
	return errors.New("boot failed")
}

func TestRegisterServiceProvidersExitsNonZeroOnRegisterError(t *testing.T) {
	if os.Getenv("HH_FOUNDATION_REGISTER_EXIT_TEST") == "1" {
		app := &Application{}
		app.RegisterServiceProviders([]service.Provider{
			failingRegisterProvider{},
		})
		os.Exit(99)
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^TestRegisterServiceProvidersExitsNonZeroOnRegisterError$")
	cmd.Env = append(os.Environ(), "HH_FOUNDATION_REGISTER_EXIT_TEST=1")

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit with a non-zero status")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected exit error, got %T: %v", err, err)
	}

	if code := exitErr.ExitCode(); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestBootServiceProvidersExitsNonZeroOnBootError(t *testing.T) {
	if os.Getenv("HH_FOUNDATION_BOOT_EXIT_TEST") == "1" {
		app := &Application{}
		app.BootServiceProviders([]service.Provider{
			failingBootProvider{},
		})
		os.Exit(99)
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^TestBootServiceProvidersExitsNonZeroOnBootError$")
	cmd.Env = append(os.Environ(), "HH_FOUNDATION_BOOT_EXIT_TEST=1")

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit with a non-zero status")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected exit error, got %T: %v", err, err)
	}

	if code := exitErr.ExitCode(); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestConfiguredServiceProvidersExitsNonZeroOnInvalidConfigType(t *testing.T) {
	if os.Getenv("HH_FOUNDATION_INVALID_PROVIDERS_EXIT_TEST") == "1" {
		original := facades.Container()
		facades.SetContainer(&facades.Services{})
		facades.Register[contractconfig.Application](fakeConfig{
			values: map[string]any{
				"kernel.providers": "invalid",
			},
		})
		defer facades.SetContainer(original)

		app := &Application{}
		app.registerConfiguredServiceProviders()
		os.Exit(99)
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^TestConfiguredServiceProvidersExitsNonZeroOnInvalidConfigType$")
	cmd.Env = append(os.Environ(), "HH_FOUNDATION_INVALID_PROVIDERS_EXIT_TEST=1")

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit with a non-zero status")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected exit error, got %T: %v", err, err)
	}

	if code := exitErr.ExitCode(); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestRegisterServiceProvidersExitsNonZeroOnNilProvider(t *testing.T) {
	if os.Getenv("HH_FOUNDATION_NIL_PROVIDER_EXIT_TEST") == "1" {
		app := &Application{}
		app.RegisterServiceProviders([]service.Provider{
			nil,
		})
		os.Exit(99)
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^TestRegisterServiceProvidersExitsNonZeroOnNilProvider$")
	cmd.Env = append(os.Environ(), "HH_FOUNDATION_NIL_PROVIDER_EXIT_TEST=1")

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit with a non-zero status")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected exit error, got %T: %v", err, err)
	}

	if code := exitErr.ExitCode(); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}
