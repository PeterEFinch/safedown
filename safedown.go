package safedown

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
)

// Order represents the order the actions will be performed relative to the
// order that they were added.
type Order uint8

const (
	FirstInLastDone   Order = iota // FirstInLastDone means that actions added first will be performed last on shutdown.
	FirstInFirstDone               // FirstInFirstDone means that actions added first will be performed first on shutdown.
	invalidOrderValue              // invalidOrderValue is a constant used to validate the value of the order.
)

// ShutdownActions represent a set of actions, i.e. functions, that are
// performed, i.e. the functions are called, when a service or process is
// shutting down, ending or interrupted.
//
// ShutdownActions must always be initialised using the NewShutdownActions
// function.
type ShutdownActions struct {
	actions []func() // actions contains the functions to be called on shutdown
	order   Order    // order represents the order actions will be performed on shutdown

	shutdownOnce *sync.Once  // shutdownOnce is used to ensure that the shutdown method is idempotent
	mutex        *sync.Mutex // mutex prevents clashes when shared across goroutines
}

// NewShutdownActions initialises shutdown actions.
//
// The parameter order determines the order the actions will be performed
// relative to the order they are added.
//
// Including signals will start a go routine which will listen for the given
// signals. If one of the signals is received the Shutdown method will be
// called. Unlike signal.Notify, using zero signals will listen to no signals
// instead of all.
func NewShutdownActions(order Order, signals ...os.Signal) *ShutdownActions {
	if order >= invalidOrderValue {
		panic("shutdown actions initialised with invalid order")
	}

	sa := &ShutdownActions{
		shutdownOnce: &sync.Once{},
		mutex:        &sync.Mutex{},
		order:        order,
	}

	sa.startListening(signals)
	return sa
}

// AddActions adds actions that will be performed when Shutdown is called.
//
// There is currently no prescribed behaviour for actions that have been added
// after the Shutdown method has been called.
func (sa *ShutdownActions) AddActions(actions ...func()) {
	// TODO: Include a strategy for adding action after shutdown has been called.

	sa.mutex.Lock()
	sa.actions = append(sa.actions, actions...)
	sa.mutex.Unlock()
}

// Shutdown will perform all actions that have been added.
//
// This is an idempotent method and successive calls will have no affect.
func (sa *ShutdownActions) Shutdown() {
	sa.shutdown()
}

func (sa *ShutdownActions) shutdown() {
	sa.shutdownOnce.Do(func() {
		sa.mutex.Lock()
		actions := sa.actions[:len(sa.actions)]
		sa.mutex.Unlock()

		if sa.order == FirstInLastDone {
			for left, right := 0, len(sa.actions)-1; left < right; left, right = left+1, right-1 {
				actions[left], actions[right] = actions[right], actions[left]
			}
		}

		for i := range actions {
			actions[i]()
		}
	})
}

func (sa *ShutdownActions) startListening(signals []os.Signal) {
	if len(signals) == 0 {
		return
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, signals...)

	go func() {
		sig := <-signalCh
		fmt.Println(sig)
		signal.Stop(signalCh)
		close(signalCh)

		sa.shutdown()
	}()
}
