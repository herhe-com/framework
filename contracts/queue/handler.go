package queue

import (
	"errors"
	"fmt"
	"time"

	"github.com/gookit/color"
)

// ErrHandlerTimeout reports that a handler exceeded its processing timeout.
var ErrHandlerTimeout = errors.New("queue: handler timed out")

// DefaultHandlerTimeout bounds one handler call when a driver does not
// configure handler_timeout.
const DefaultHandlerTimeout = 10 * time.Minute

// CallHandler runs the handler with the given maximum processing time. A
// timeout is returned as an error wrapping ErrHandlerTimeout, so the message
// follows the driver's normal retry or error path. A timeout of zero or less
// disables the limit. driver labels the timeout log line, e.g. "rabbitmq".
//
// Go cannot stop the abandoned goroutine: the handler keeps running in the
// background while its message is retried. Handlers should still enforce their
// own context deadlines.
func CallHandler(driver string, handler Handler, data []byte, timeout time.Duration) (any, error) {
	if handler == nil {
		return nil, errors.New("queue: handler is required")
	}
	if timeout <= 0 {
		return handler(data)
	}

	type result struct {
		response any
		err      error
	}

	done := make(chan result, 1)

	go func() {
		response, err := handler(data)
		done <- result{response: response, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case out := <-done:
		return out.response, out.err
	case <-timer.C:
		err := fmt.Errorf("%w after %s", ErrHandlerTimeout, timeout)
		color.Errorf(
			"[queue.%s] %v; the message is retried while the handler keeps running in the background\n",
			driver,
			err,
		)

		return nil, err
	}
}
