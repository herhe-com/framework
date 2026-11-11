package nats

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// minimumScheduleVersion is the first nats-server release with JetStream
// message schedules ($JS schedule, allow_msg_schedules).
const minimumScheduleVersion = "2.12.0"

// requireScheduleSupport fails when the connected broker predates message
// schedules. Older servers accept the stream flag, log a warning and drop it,
// so a delayed message would be accepted and never delivered; failing at
// startup keeps that silent loss from happening.
func (r *NATS) requireScheduleSupport() error {
	if r.conn == nil || r.conn.IsClosed() {
		return natsgo.ErrConnectionClosed
	}

	version := r.conn.ConnectedServerVersion()
	if serverVersionAtLeast(version, 2, 12) {
		return nil
	}

	return fmt.Errorf(
		"nats: server version %q is too low for delayed queues (message schedules); nats-server v%s or later is required",
		version,
		minimumScheduleVersion,
	)
}

// serverVersionAtLeast reports whether a nats-server version string like
// "2.12.1" or "2.12.1-RC.2" is at least wantMajor.wantMinor.
func serverVersionAtLeast(version string, wantMajor, wantMinor int) bool {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")

	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}

	major, err := strconv.Atoi(leadingDigits(parts[0]))
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(leadingDigits(parts[1]))
	if err != nil {
		return false
	}

	if major != wantMajor {
		return major > wantMajor
	}

	return minor >= wantMinor
}

// leadingDigits strips a prerelease suffix from a version segment.
func leadingDigits(segment string) string {
	for index, char := range segment {
		if char < '0' || char > '9' {
			return segment[:index]
		}
	}

	return segment
}

// requireSchedules fails when the broker did not really enable message
// schedules. Servers older than v2.12 accept the stream flag, log a warning and
// silently drop it, which would turn every delayed message into a message that
// nothing ever delivers.
func requireSchedules(ctx context.Context, stream jetstream.Stream, name string) error {
	info, err := stream.Info(ctx)
	if err != nil {
		return fmt.Errorf("nats: inspect stream %s: %w", name, err)
	}
	if !info.Config.AllowMsgSchedules {
		return fmt.Errorf(
			"nats: stream %s does not allow message schedules; delayed queues require nats-server v2.12.0 or later",
			name,
		)
	}

	return nil
}
