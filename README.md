# Safedown

Safedown is for ensuring that applications shutdown gracefully and correctly. This includes the cases when an interrupt
or termination signal is received, or when actions across go routines need to be coordinated.

<p align="center">
<em>
Safedown is like defer but more graceful and coordinated.
</em>
</p>

### Quick Start

Adding shutdown actions along with a set of signals allows for methods (in this case `cancel`) to be run when a
termination signal, or similar, is received.

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/PeterEFinch/safedown"
)

func main() {
	// Safedown actions must be initiated with an order as well as signals
	// that should be listened for. Including `defer sa.Shutdown()` is not 
	// necessary but can be included to ensure that the actions are run at the
	// very end even if no signal is received.
	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa.SetPostShutdownStrategy(safedown.PerformImmediately)
	defer sa.Shutdown()

	// Setting this function will allow logic based on what signal was 
	// received.
	sa.SetOnSignal(func(signal os.Signal) {
		fmt.Printf("Signal received: %s\n", signal.String())
	})

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
```

### F.A.Q. (Fictitiously Asked Questions)

1. *What signals should I listen for?*
   This depends on which OS is being used. For example, for code in docker images running alpine listening for at
   least `syscall.SIGTERM` & `syscall.SIGINT` is recommended.

2. *What order should I used?*
   First in, last done order is most commonly used because the first things create are usually the last ones that need
   tearing down. This matches the view that safedown is like defer but graceful and coordinated.

3. *What post shutdown strategy should I use?*
   This depends on the application and how it is initialised. The post shutdown strategy is usually only used when the
   application is interrupted during its initialisation.

4. *What OSes has this been test on?*
   The tests have been run locally on macOS Monterey and on ubuntu in the github actions. It has not been tested on
   Windows, OpenBSD, etc.

5. *Why are there no dependencies?*
   This repository is intended to be a zero-dependency library. This makes it easier to maintain and prevents adding
   external vulnerabilities or bugs.

6. *Why is there no logging?*
   There is no convention when it comes to logging, so it was considered best to avoid it. The code is simple enough
   that it seems unnecessary.

7. *Can I use this in mircoservices?*
   Yes. This was original designed to ensure graceful shutdown in microservices.

8. *Why is there another VERY similar Safedown?*
   I originally wrote a version of safedown as package in personal project, which I rewrote inside a Graphmasters
   service (while I was an employee), which I finally put inside its
   own [Graphmasters repository](github.com/Graphmasters/safedown) (which I as of writing this I still maintain).
   Graphmasters and I decided they would make their version open source (yay) and I decided to reimplement my own
   version from scratch with ideas from the original version because I wanted to.

   It doesn't really matter which version you used as both version are very similar. The Graphmasters version is stable
   and used in production microservices. This version currently has a few additional features.