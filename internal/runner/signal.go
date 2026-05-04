package runner

import (
	"os"
	"os/signal"
	"syscall"
)

// trapSIGINT returns a channel that receives when SIGINT is received.
func trapSIGINT() <-chan struct{} {
	ch := make(chan struct{}, 1)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() {
		<-sigs
		ch <- struct{}{}
	}()
	return ch
}
