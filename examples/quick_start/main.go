package main

import (
	"context"
	"fmt"
	"time"

	"github.com/PeterEFinch/safedown"
)

func main() {
	// Shutdown actions must be initialised using the constructor.
	sa := safedown.NewShutdownActions(
		safedown.UseOrder(safedown.FirstInLastDone), // This option is unnecessary because it is the default.
		safedown.UsePostShutdownStrategy(safedown.PerformImmediately),
		safedown.ShutdownOnAnySignal(),
	)

	// Including `defer sa.Shutdown()` is not necessary but can be included to
	// ensure that the actions are run at the very end even if no signal is
	// received.
	defer sa.Shutdown()

	// The cancel can be added so that it is ensured that it is always called.
	// In the case `defer sa.Shutdown()` wasn't included, defer cancel() should
	// be added.
	ctx, cancel := context.WithCancel(context.Background())
	sa.AddActions(cancel)

	// This code is just a stand in for a general process.
	fmt.Println("Processing starting")
	t := time.After(time.Minute)
	select {
	case <-ctx.Done():
	case <-t:
	}
	fmt.Println("Finished")
}
