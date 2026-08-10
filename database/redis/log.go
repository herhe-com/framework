package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	redisLogModeSilent = "silent"
	redisLogModeError  = "error"
	redisLogModeInfo   = "info"
)

type requestLogHook struct {
	connection string
	mode       string
	logger     *log.Logger
}

var _ goredis.Hook = (*requestLogHook)(nil)

func newRequestLogHook(connection, mode string) *requestLogHook {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == redisLogModeSilent {
		return nil
	}
	if mode != redisLogModeInfo {
		mode = redisLogModeError
	}

	return &requestLogHook{
		connection: connection,
		mode:       mode,
		logger:     log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (h *requestLogHook) DialHook(next goredis.DialHook) goredis.DialHook {
	return next
}

func (h *requestLogHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		startedAt := time.Now()
		err := next(ctx, cmd)
		h.write(cmd.FullName(), -1, time.Since(startedAt), err)

		return err
	}
}

func (h *requestLogHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		startedAt := time.Now()
		err := next(ctx, cmds)
		h.write("pipeline", len(cmds), time.Since(startedAt), err)

		return err
	}
}

func (h *requestLogHook) write(command string, commandCount int, elapsed time.Duration, err error) {
	if h.mode != redisLogModeInfo && (err == nil || errors.Is(err, goredis.Nil)) {
		return
	}

	status := "ok"
	if errors.Is(err, goredis.Nil) {
		status = "miss"
	} else if err != nil {
		status = "error"
	}

	var message strings.Builder
	fmt.Fprintf(&message, "[redis] connection=%q command=%q", h.connection, command)
	if commandCount >= 0 {
		fmt.Fprintf(&message, " commands=%d", commandCount)
	}
	fmt.Fprintf(&message, " duration=%s status=%s", elapsed.Round(time.Microsecond), status)
	if status == "error" {
		fmt.Fprintf(&message, " error=%q", err)
	}

	h.logger.Print(message.String())
}
