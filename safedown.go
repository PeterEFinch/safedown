package safedown

import (
	"sync"
)

type ShutdownActions struct {
	actions []func()   // actions contains the functions to be run on shutdown
	mutex   sync.Mutex // mutex prevents clashes when shared across goroutines
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
	/*
		Shutdown logic was separated into its own method in anticipation that
		there will be multiple ways of triggering the shutdown actions.
	*/

	sa.mutex.Lock()
	actions := sa.actions[:]
	sa.mutex.Unlock()

	for i := range actions {
		actions[i]()
	}
}
