package redis

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestRequestLogHookInfoLogsMetadataWithoutArguments(t *testing.T) {
	var output bytes.Buffer
	hook := newRequestLogHook("default", redisLogModeInfo)
	hook.logger = log.New(&output, "", 0)

	cmd := goredis.NewStatusCmd(context.Background(), "set", "session:1", "secret-value")
	process := hook.ProcessHook(func(context.Context, goredis.Cmder) error {
		return nil
	})

	if err := process(context.Background(), cmd); err != nil {
		t.Fatalf("expected request to succeed, got %v", err)
	}

	got := output.String()
	for _, want := range []string{`connection="default"`, `command="set"`, "duration=", "status=ok"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected log to contain %q, got %q", want, got)
		}
	}
	for _, sensitive := range []string{"session:1", "secret-value"} {
		if strings.Contains(got, sensitive) {
			t.Fatalf("expected log to omit command argument %q, got %q", sensitive, got)
		}
	}
}

func TestRequestLogHookErrorMode(t *testing.T) {
	t.Run("skips cache misses", func(t *testing.T) {
		var output bytes.Buffer
		hook := newRequestLogHook("default", redisLogModeError)
		hook.logger = log.New(&output, "", 0)

		process := hook.ProcessHook(func(context.Context, goredis.Cmder) error {
			return goredis.Nil
		})
		if err := process(context.Background(), goredis.NewStringCmd(context.Background(), "get", "missing")); !errors.Is(err, goredis.Nil) {
			t.Fatalf("expected redis.Nil, got %v", err)
		}
		if got := output.String(); got != "" {
			t.Fatalf("expected no log for redis.Nil, got %q", got)
		}
	})

	t.Run("logs failures", func(t *testing.T) {
		var output bytes.Buffer
		hook := newRequestLogHook("cache", redisLogModeError)
		hook.logger = log.New(&output, "", 0)
		requestErr := errors.New("connection closed")

		process := hook.ProcessHook(func(context.Context, goredis.Cmder) error {
			return requestErr
		})
		if err := process(context.Background(), goredis.NewStringCmd(context.Background(), "get", "key")); !errors.Is(err, requestErr) {
			t.Fatalf("expected original error, got %v", err)
		}

		got := output.String()
		for _, want := range []string{`connection="cache"`, `command="get"`, "status=error", `error="connection closed"`} {
			if !strings.Contains(got, want) {
				t.Fatalf("expected log to contain %q, got %q", want, got)
			}
		}
	})
}

func TestRequestLogHookPipelineLogsOnlyCommandCount(t *testing.T) {
	var output bytes.Buffer
	hook := newRequestLogHook("default", redisLogModeInfo)
	hook.logger = log.New(&output, "", 0)
	cmds := []goredis.Cmder{
		goredis.NewStatusCmd(context.Background(), "set", "private:key", "private-value"),
		goredis.NewStringCmd(context.Background(), "get", "another:key"),
	}

	process := hook.ProcessPipelineHook(func(context.Context, []goredis.Cmder) error {
		return nil
	})
	if err := process(context.Background(), cmds); err != nil {
		t.Fatalf("expected pipeline to succeed, got %v", err)
	}

	got := output.String()
	for _, want := range []string{`command="pipeline"`, "commands=2", "status=ok"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected log to contain %q, got %q", want, got)
		}
	}
	for _, sensitive := range []string{"private:key", "private-value", "another:key"} {
		if strings.Contains(got, sensitive) {
			t.Fatalf("expected pipeline log to omit command argument %q, got %q", sensitive, got)
		}
	}
}

func TestNewRequestLogHookModes(t *testing.T) {
	if hook := newRequestLogHook("default", redisLogModeSilent); hook != nil {
		t.Fatal("expected silent mode to disable request logging")
	}

	hook := newRequestLogHook("default", "unknown")
	if hook.mode != redisLogModeError {
		t.Fatalf("expected unknown mode to fall back to error, got %q", hook.mode)
	}
}
