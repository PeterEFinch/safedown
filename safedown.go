/*
Package safedown is for ensuring that applications shutdown gracefully and
correctly. This includes the cases when an interrupt or termination signal is
received, or when actions across go routines need to be coordinated.
*/
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

// PostShutdownStrategy represents the strategy that should be applied to action
// added after shutdown has been triggered.
type PostShutdownStrategy uint8

const (
	DoNothing                       PostShutdownStrategy = iota // DoNothing means that any action added after shutdown has been trigger will not be done.
	PerformImmediately                                          // PerformImmediately means that any action added after shutdown will be performed immediately (and block the AddAction method).
	PerformImmediatelyInBackground                              // PerformImmediatelyInBackground means that any action added after shutdown will be performed immediately in a go routine.
	PerformCoordinatelyInBackground                             // PerformCoordinatelyInBackground means that the shutdown actions will ATTEMPT to coordinate the actions added with all other actions which have already been added.
)

// ShutdownActions represent a set of actions that are performed, i.e. functions
// that are called, when a service or computation is shutting down, ending or
// interrupted.
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
// In the event that options conflict, the later option will override the
// earlier option.
func NewShutdownActions(options ...Option) *ShutdownActions {
	config := &config{}
	for _, option := range options {
		option(config)
	}

	sa := &ShutdownActions{
		order:        config.order,
		onSignalFunc: config.onSignalFunc,
		strategy:     config.strategy,

		mutex:             &sync.Mutex{},
		shutdownCh:        make(chan struct{}),
		shutdownOnce:      &sync.Once{},
		stopListeningCh:   make(chan struct{}),
		stopListeningOnce: &sync.Once{},
	}

	sa.startListening(config.shutdownOnAnySignal, config.shutdownOnSignals)
	return sa
}

// AddActions adds actions that are to be performed when Shutdown is called.
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
		sa.performActions(actions)
		return
	case PerformImmediatelyInBackground:
		sa.mutex.Unlock()
		go sa.performActions(actions)
		return
	case PerformCoordinatelyInBackground:
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

// Shutdown will perform all actions that have been added.
//
// This is an idempotent method and successive calls will have no affect.
func (sa *ShutdownActions) Shutdown() {
	sa.stopListening()
	sa.shutdown()
}

// Wait waits until the shutdown actions to have been performed.
//
// If actions were added after Shutdown has been called or trigger via a signal
// then whether this method wait for those actions to be performed on the
// post-shutdown strategy and when the actions were added.
func (sa *ShutdownActions) Wait() {
	<-sa.shutdownCh
}

func (sa *ShutdownActions) onSignal(received os.Signal) {
	if received == nil || sa.onSignalFunc == nil {
		return
	}

	sa.onSignalFunc(received)
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

func (sa *ShutdownActions) startListening(shutdownOnAnySignal bool, signals []os.Signal) {
	if !shutdownOnAnySignal && len(signals) == 0 {
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

// config represents configuration for initialising the shutdown actions.
type config struct {
	order               Order                // order represents the order actions will be performed on shutdown
	onSignalFunc        func(os.Signal)      // onSignalFunc gets called if a signal is received
	strategy            PostShutdownStrategy // strategy contains the post shutdown strategy
	shutdownOnAnySignal bool                 // shutdownOnAnySignal indicates if the shutdown should be triggered by any signal
	shutdownOnSignals   []os.Signal          // shutdownOnSignals stores the specific signals that should trigger shutdown
}

// Option represents an option of the shutdown actions.
type Option func(*config)

// ShutdownOnAnySignal will enable shutdown to be triggered by any signal. To
// trigger shutdown by a limited number of signals use ShutdownOnSignals.
//
// This option will override ShutdownOnSignals if included after it.
func ShutdownOnAnySignal() Option {
	return func(o *config) {
		o.shutdownOnSignals = nil
		o.shutdownOnAnySignal = true
	}
}

// ShutdownOnSignals will enable the shutdown to be triggered by any of the
// signals included. To trigger shutdown on any signal use the option
// ShutdownOnAnySignal.
//
// This option will override ShutdownOnAnySignal if included after it.
func ShutdownOnSignals(signals ...os.Signal) Option {
	return func(o *config) {
		o.shutdownOnSignals = signals
		o.shutdownOnAnySignal = false
	}
}

// UseOnSignalFunc sets a function that will be called that if any signal that
// is listened for is received.
func UseOnSignalFunc(onSignal func(os.Signal)) Option {
	return func(o *config) {
		o.onSignalFunc = onSignal
	}
}

// UseOrder determines the order the actions will be performed relative to the
// order they are added.
func UseOrder(order Order) Option {
	if order >= invalidOrderValue {
		panic("shutdown option UseOrder set with invalid order")
	}

	return func(o *config) {
		o.order = order
	}
}

// UsePostShutdownStrategy determines how the actions will be handled after
// Shutdown has been called or triggered via a signal.
//
// The strategy is usually only used when the Shutdown has been triggered during
// the initialisation of an application.
func UsePostShutdownStrategy(strategy PostShutdownStrategy) Option {
	return func(o *config) {
		o.strategy = strategy
	}
}
