package runner

import (
	"os"
	"os/signal"
	"syscall"
)

// trapSIGINT returns a channel that receives when SIGINT is received.
// The returned stop function should be called to clean up the signal handler.
func trapSIGINT() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() {
		<-sigs
		ch <- struct{}{}
	}()
	return ch, func() { signal.Stop(sigs) }
}
