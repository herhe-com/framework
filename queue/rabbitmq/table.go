package rabbitmq

import (
	"errors"

	amqp "github.com/rabbitmq/amqp091-go"
)

// tableFromMap converts loosely typed configuration into an AMQP table.
func tableFromMap(values map[string]any) amqp.Table {
	if len(values) == 0 {
		return nil
	}

	table := make(amqp.Table, len(values))
	for key, value := range values {
		table[key] = value
	}

	return table
}

// mergeTables returns base overridden by overrides.
func mergeTables(base, overrides amqp.Table) amqp.Table {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}

	merged := make(amqp.Table, len(base)+len(overrides))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}

	return merged
}

// isClosedError reports whether err only means "the channel or connection is
// already gone", which is expected while shutting down or recovering.
func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, amqp.ErrClosed) {
		return true
	}

	var amqpErr *amqp.Error
	if errors.As(err, &amqpErr) {
		switch amqpErr.Code {
		case amqp.ChannelError, amqp.FrameError, amqp.ConnectionForced, amqp.NotImplemented:
			return true
		}
	}

	return false
}
