package foundation

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/herhe-com/framework/contracts/service"
)

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
