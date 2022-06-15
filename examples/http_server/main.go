package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/PeterEFinch/safedown"
)

func main() {
	// Initialising shutdown actions. Any actioned added is called if a signal
	// is received.
	sa := safedown.NewShutdownActions(
		safedown.UseOrder(safedown.FirstInLastDone), // This option is unnecessary because it is the default.
		safedown.UsePostShutdownStrategy(safedown.PerformImmediately),
		safedown.ShutdownOnSignals(syscall.SIGTERM, syscall.SIGINT), // Replace with OS specific signals.
		safedown.UseOnSignalFunc(func(signal os.Signal) {
			log.Printf("Signal received: %s\n", signal.String())
		}),
	)
	defer sa.Shutdown()

	server := &http.Server{
		Addr: "address",
		Handler: http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			// Do something ...
		}),
	}
	sa.AddActions(func() {
		// The timeout and error handling should be set by the user.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("http server failed to shutdown: %v\n", err)
		}
	})

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Printf("http server encountered error: %v\n", err)
	}
}
