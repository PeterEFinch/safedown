package safedown

import (
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

// PostShutdownStrategy represent the strategy that should be applied to action
// added after shutdown has been triggered.
type PostShutdownStrategy uint8

const (
	DoNothing           PostShutdownStrategy = iota // DoNothing means that any action added after shutdown has been trigger will not be done.
	PerformImmediately                              // PerformImmediately means that any action added after shutdown will be performed immediately.
	PerformCoordinately                             // PerformCoordinately means that the shutdown actions will ATTEMPT to coordinate these actions as much as possible.
)

// ShutdownActions represent a set of actions, i.e. functions, that are
// performed, i.e. the functions are called, when a service or process is
// shutting down, ending or interrupted.
//
// ShutdownActions must always be initialised using the NewShutdownActions
// function.
type ShutdownActions struct {
	actions      []func()             // actions contains the functions to be called on shutdown
	order        Order                // order represents the order actions will be performed on shutdown
	onSignalFunc func(os.Signal)      // onSignalFunc gets called if a signal is received
	strategy     PostShutdownStrategy // strategy contains the post shutdown strategy

	mutex               *sync.Mutex   // mutex prevents clashes when shared across goroutines
	isProcessingActive  bool          // isProcessingActive is true if and only if the stored actions are being performed or just about to be performed
	isProcessingAllowed bool          // isProcessingAllowed is true if and only if actions can be performed when added (occurs after shutdown has been triggered)
	shutdownCh          chan struct{} // shutdownCh will be closed when shutdown has been completed
	shutdownOnce        *sync.Once    // shutdownOnce is used to ensure that the shutdown method is idempotent
	stopListeningCh     chan struct{} // stopListeningCh can be closed to indicate that signals should no longer be listened for
	stopListeningOnce   *sync.Once    // stopListeningOnce is used to ensure that stopListeningCh is closed at most once
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
		mutex:             &sync.Mutex{},
		order:             order,
		shutdownCh:        make(chan struct{}),
		shutdownOnce:      &sync.Once{},
		stopListeningCh:   make(chan struct{}),
		stopListeningOnce: &sync.Once{},
	}

	sa.startListening(signals)
	return sa
}

// AddActions adds actions that will be performed when Shutdown is called.
//
// If Shutdown has already been called or trigger via a signal then the handling
// of the actions will depend on the post-shutdown strategy.
func (sa *ShutdownActions) AddActions(actions ...func()) {
	sa.mutex.Lock()
	// This is the pre-shutdown phase
	if !sa.isProcessingAllowed {
		sa.actions = append(sa.actions, actions...)
		sa.mutex.Unlock()
		return
	}

	// This is the post-shutdown phase i.e. shutdown has been called but not
	// necessarily completed.
	switch sa.strategy {
	case PerformImmediately:
		sa.mutex.Unlock()
		go sa.performActions(actions)
		return
	case PerformCoordinately:
		sa.actions = append(sa.actions, actions...)
		if sa.isProcessingActive {
			sa.mutex.Unlock()
			return
		}

		sa.isProcessingActive = true
		sa.mutex.Unlock()
		go sa.performStoredActions()
		return
	default:
		sa.mutex.Unlock()
		return
	}
}

// SetOnSignal sets a function that will be called if a signal is received.
func (sa *ShutdownActions) SetOnSignal(onSignal func(os.Signal)) {
	sa.mutex.Lock()
	sa.onSignalFunc = onSignal
	sa.mutex.Unlock()
}

// SetPostShutdownStrategy determines how the actions will be handled after
// Shutdown has been called or triggered via a signal.
func (sa *ShutdownActions) SetPostShutdownStrategy(strategy PostShutdownStrategy) {
	sa.mutex.Lock()
	sa.strategy = strategy
	sa.mutex.Unlock()
}

// Shutdown will perform all actions that have been added.
//
// This is an idempotent method and successive calls will have no affect.
func (sa *ShutdownActions) Shutdown() {
	sa.shutdown()
	sa.stopListening()
}

// Wait waits for the shutdown actions to have been performed.
func (sa *ShutdownActions) Wait() {
	<-sa.shutdownCh
}

func (sa *ShutdownActions) onSignal(received os.Signal) {
	if received == nil {
		return
	}

	sa.mutex.Lock()
	onSignal := sa.onSignalFunc
	sa.mutex.Unlock()

	if onSignal == nil {
		return
	}

	onSignal(received)
}

func (sa *ShutdownActions) performActions(actions []func()) {
	if sa.order == FirstInLastDone {
		for left, right := 0, len(actions)-1; left < right; left, right = left+1, right-1 {
			actions[left], actions[right] = actions[right], actions[left]
		}
	}

	for i := range actions {
		actions[i]()
	}
}

func (sa *ShutdownActions) performStoredActions() {
	// To avoid this method being called multiple times in concurrent go
	// routines the boolean isProcessingActive MUST be change from FALSE to TRUE
	// in a concurrent safe manner prior to calling this method.
	for {
		var action func()
		sa.mutex.Lock()
		switch {
		case len(sa.actions) == 0:
			sa.isProcessingActive = false
			sa.mutex.Unlock()
			return
		case sa.order == FirstInLastDone:
			action = sa.actions[len(sa.actions)-1]
			sa.actions = sa.actions[:len(sa.actions)-1]
		default:
			action = sa.actions[0]
			sa.actions = sa.actions[1:]
		}
		sa.mutex.Unlock()

		action()
	}
}

func (sa *ShutdownActions) shutdown() {
	sa.shutdownOnce.Do(func() {
		sa.mutex.Lock()
		sa.isProcessingAllowed = true
		sa.isProcessingActive = true
		sa.mutex.Unlock()

		sa.performStoredActions()
		close(sa.shutdownCh)
	})
}

func (sa *ShutdownActions) startListening(signals []os.Signal) {
	if len(signals) == 0 {
		sa.stopListening()
		return
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, signals...)

	go func() {
		var received os.Signal
		select {
		case <-sa.stopListeningCh:
		case received = <-signalCh:
		}

		signal.Stop(signalCh)
		close(signalCh)

		sa.onSignal(received)
		sa.Shutdown()
	}()
}

func (sa *ShutdownActions) stopListening() {
	sa.stopListeningOnce.Do(func() {
		close(sa.stopListeningCh)
	})
}
