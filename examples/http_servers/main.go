package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/PeterEFinch/safedown"
)

// main demonstrates how a useful way to initialise two critical HTTP servers.
// The servers are critical in the sense that if either permanently stop the
// application should stop.
//
// There are instances where having two HTTP servers is useful such as one
// requiring auth and the other not, e.g. for health. Alternatively, one of the
// HTTP servers could be viewed as a stand-in for another type of server, e.g.
// gRPC.
func main() {
	sa := safedown.NewShutdownActions(
		safedown.UseOrder(safedown.FirstInLastDone), // This option is unnecessary because it is the default.
		safedown.UsePostShutdownStrategy(safedown.PerformImmediately),
		safedown.ShutdownOnAnySignal(),
		safedown.UseOnSignalFunc(func(signal os.Signal) {
			log.Printf("Signal received: %s\n", signal.String())
		}),
	)

	// We initialise a wait group and add the number of goroutines.
	wg := &sync.WaitGroup{}
	wg.Add(2)

	go startHTTPServerA(wg, sa, "address_auth_required")
	go startHTTPServerB(wg, sa, "address_no_auth_required")

	// Wait waits for all the goroutines to have ended.
	wg.Wait()

}

func startHTTPServerA(wg *sync.WaitGroup, sa *safedown.ShutdownActions, address string) {
	// In situations when a server is considered critical, it is recommended
	// that shutdown is deferred in case the server stops for any reason.
	defer wg.Done()
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

func startHTTPServerB(wg *sync.WaitGroup, sa *safedown.ShutdownActions, address string) {
	// In situations when a server is considered critical, it is recommended
	// that shutdown is deferred in case the server stops for any reason.
	defer wg.Done()
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
