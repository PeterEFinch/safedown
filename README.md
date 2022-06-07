# Safedown

Safedown is for ensuring that applications shutdown gracefully and correctly. This includes the cases when an interrupt
or termination signal is received, or when actions across go routines need to be coordinated.

<p align="center">
<em>
Safedown is like defer but more graceful and coordinated.
</em>
</p>

### Quick Start

The shutdown actions are initialised through its constructor. Methods are added (in this case `cancel`) which will be
run when a signal is received or when the `Shutdown` method is called.

```go
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
```

For more detailed examples see the [examples module](./examples).

### F.A.Q. (Fictitiously Asked Questions)

1. *What order should I used?*
   First in, last done order is most commonly used because the first things create are usually the last ones that need
   tearing down. This matches the view that safedown is like defer but graceful and coordinated.

2. *What signals should I listen for?*
   This depends on which OS is being used. For example, for code in docker images running alpine listening for at
   least `syscall.SIGTERM` & `syscall.SIGINT` is recommended. If unsure listening for any signal is reasonable, however,
   not all signals will be caught e.g `os.Kill` isn't caught on ubuntu.

3. *Should I use a post shutdown strategy?*
   This depends on the application and how it is initialised. The post shutdown strategy is usually only used when the
   application is interrupted during its initialisation.

4. *What OSes has this been test on?*
   The tests have been successfully run locally on macOS Monterey and on ubuntu in the github actions. Tests failed on
   Windows in github actions because `os.Interrupt` has not been implemented. It not has not been tested on other OSes
   such as OpenBSD.

7. *Why are there no dependencies?*
   This repository is intended to be a zero-dependency library. This makes it easier to maintain and prevents adding
   external vulnerabilities or bugs.

8. *Why is there no logging?*
   There is no convention when it comes to logging, so it was considered best to avoid it. The code is simple enough
   that it seems unnecessary.

9. *Can I use this in mircoservices?*
   Yes. This was original designed to ensure graceful shutdown in microservices.

10. *Why is there another similar Safedown?*
    I originally wrote a version of safedown as package in personal project, which I rewrote inside a Graphmasters
    service (while I was an employee), which I finally put inside its
    own [Graphmasters Safedown repository](https://github.com/Graphmasters/safedown) (which I as of writing this I still
    maintain). Graphmasters and I decided they would make their version open source (yay) and I decided to reimplement
    my own version from scratch with ideas from the original version because I wanted to expand upon some ideas.