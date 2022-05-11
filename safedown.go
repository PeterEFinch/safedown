package safedown

import (
	"sync"
)

// ShutdownActions represent a set of actions, i.e. functions, that are
// performed, i.e. the functions are called, when a service or process is
// shutting down, ending or interrupted.
//
// ShutdownActions must always be initialised using the NewShutdownActions
// function.
type ShutdownActions struct {
	actions []func() // actions contains the functions to be called on shutdown

	shutdownOnce *sync.Once  // shutdownOnce is used to ensure that the shutdown method is idempotent
	mutex        *sync.Mutex // mutex prevents clashes when shared across goroutines
}

// NewShutdownActions initialises shutdown actions.
func NewShutdownActions() *ShutdownActions {
	return &ShutdownActions{
		shutdownOnce: &sync.Once{},
		mutex:        &sync.Mutex{},
	}
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
		actions := sa.actions[:]
		sa.mutex.Unlock()

		for i := range actions {
			actions[i]()
		}
	})
}
