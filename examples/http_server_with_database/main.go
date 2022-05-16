package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dgraph-io/badger/v3"

	"github.com/PeterEFinch/safedown"
)

func main() {
	// This sets up the shutdown actions.
	//
	// Using "safedown.FirstInLastDone" means that the HTTP server will stop
	// and then database will stop. This means HTTP calls that rely on the
	// database will still have access to the database.
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
	defer sa.Shutdown()

	// The database was chosen to be badger out of convenience.
	db, err := badger.Open(badger.DefaultOptions("/path/to/badger/file"))
	if err != nil {
		// The action on error should be chosen by the user. It isn't recommended
		// to call shutdown in this case because there is no guaranteed for
		// behaviour of actions added once shutting down has begun.
		log.Fatalf("badger database failed to open: %v\n", err)
	}
	sa.AddActions(func() {
		// The error handling should be set by the user.
		if err := db.Close(); err != nil {
			log.Printf("badger database failed to close: %v\n", err)
		}
	})

	// Creates a mux server.
	mux := http.NewServeMux()
	mux.HandleFunc("something", func(_ http.ResponseWriter, _ *http.Request) {
		// Do something with the request that relies on the database...
	})

	// This sets up an HTTP server.
	server := &http.Server{
		Addr:    "address",
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

	// The HTTP server starts
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Printf("http server encountered error: %v\n", err)
	}
}
