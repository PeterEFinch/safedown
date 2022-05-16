package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/PeterEFinch/safedown"
)

// main demonstrates how a useful way to initialise two HTTP servers. There are
// instances where having two HTTP servers is useful such as one requiring auth
// and the other not, e.g. for health. Alternatively, one of the HTTP servers
// could be viewed as a stand-in for another type of server, e.g. gRPC.
func main() {
	// This sets up the shutdown actions.
	//
	// The signals were chosen as they are common interrupt signals
	sa := safedown.NewShutdownActions(
		safedown.UseOrder(safedown.FirstInLastDone), // This option is unnecessary because it is the default.
		safedown.UsePostShutdownStrategy(safedown.PerformImmediately),
		safedown.ShutdownOnAnySignal(),
		safedown.UseOnSignalFunc(func(signal os.Signal) {
			log.Printf("Signal received: %s\n", signal.String())
		}),
	)

	go startHTTPServerA(sa, "address_auth_required")
	go startHTTPServerB(sa, "address_no_auth_required")

	// Wait will ensure that the service keeps running until shutdown has been
	// completed.
	sa.Wait()

}

func startHTTPServerA(sa *safedown.ShutdownActions, address string) {
	// In situations when a server is considered critical, it is recommended
	// that shutdown is deferred in case the server stop for any reason.
	defer sa.Shutdown()

	mux := http.NewServeMux()
	mux.HandleFunc("pattern", func(_ http.ResponseWriter, _ *http.Request) {
		// Do something ...
	})

	server := &http.Server{
		Addr:    address,
		Handler: mux,
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

func startHTTPServerB(sa *safedown.ShutdownActions, address string) {
	// In situations when a server is considered critical, it is recommended
	// that shutdown is deferred in case the server stop for any reason.
	defer sa.Shutdown()

	mux := http.NewServeMux()
	mux.HandleFunc("pattern", func(_ http.ResponseWriter, _ *http.Request) {
		// Do something ...
	})

	server := &http.Server{
		Addr:    address,
		Handler: mux,
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
