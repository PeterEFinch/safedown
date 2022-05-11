package safedown

import (
	"sync"
)

type ShutdownActions struct {
	actions []func() // actions contains the functions to be run on shutdown

	shutdownOnce *sync.Once  // shutdownOnce is used to ensure that the shutdown method is idempotent
	mutex        *sync.Mutex // mutex prevents clashes when shared across goroutines
}

func NewShutdownActions() *ShutdownActions {
	return &ShutdownActions{
		shutdownOnce: &sync.Once{},
		mutex:        &sync.Mutex{},
	}
}

func (sa *ShutdownActions) AddActions(actions ...func()) {
	sa.mutex.Lock()
	sa.actions = append(sa.actions, actions...)
	sa.mutex.Unlock()
}

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
