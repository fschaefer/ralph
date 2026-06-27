package runner

import (
	"context"
	"os/signal"
	"syscall"
)

// notifyContext returns a context that is cancelled on SIGINT.
// The stop function restores the original signal handler.
func notifyContext() (context.Context, func()) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT)
}
