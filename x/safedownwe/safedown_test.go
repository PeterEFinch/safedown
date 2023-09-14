package safedownwe_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PeterEFinch/safedown/x/safedownwe"
)

// region Examples

// Example demonstrates how setting up the safedownwe's shutdown actions works
// when a signal is received.
func Example() {
	// This will send an interrupt signal after a second to simulate a signal
	// being sent from the outside.
	go func(pid int) {
		time.Sleep(time.Second)
		process := os.Process{Pid: pid}
		if err := process.Signal(os.Interrupt); err != nil {
			panic("unable to continue test: could not send signal to process")
		}
	}(os.Getpid())

	sa := safedownwe.NewShutdownActions(
		safedownwe.ShutdownOnSignals(os.Interrupt),
		safedownwe.UseOnSignalFunc(func(signal os.Signal) {
			fmt.Printf("Signal received: %s\n", signal.String())
		}),
	)
	defer sa.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	sa.AddActions(cancel)

	fmt.Println("Processing starting")
	t := time.After(2 * time.Second)
	select {
	case <-ctx.Done():
		fmt.Println("Context cancelled")
	case <-t:
		fmt.Println("Ticker ticked")
	}
	fmt.Println("Finished")

	// Output:
	// Processing starting
	// Signal received: interrupt
	// Context cancelled
	// Finished
}

// Example_signalNotReceived demonstrates how setting up the safedownwe's
// shutdown actions works when no signal is received (and the program can
// terminate of its own accord).
func Example_noSignal() {
	sa := safedownwe.NewShutdownActions(
		safedownwe.ShutdownOnSignals(os.Interrupt),
		safedownwe.UseOnSignalFunc(func(signal os.Signal) {
			fmt.Printf("Signal received: %s\n", signal.String())
		}),
	)
	defer sa.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	sa.AddActions(cancel)

	fmt.Println("Processing starting")
	t := time.After(2 * time.Second)
	select {
	case <-ctx.Done():
		fmt.Println("Context cancelled")
	case <-t:
		fmt.Println("Ticker ticked")
	}
	fmt.Println("Finished")

	// Output:
	// Processing starting
	// Ticker ticked
	// Finished
}

// ExampleShutdownActions_Shutdown demonstrates the default
// shutdown behaviour.
func ExampleShutdownActions_Shutdown() {
	sa := safedownwe.NewShutdownActions()

	sa.AddActions(func() {
		fmt.Println("The action is performed after shutdown is called.")
	})

	fmt.Println("Code runs before shutdown is called.")
	sa.Shutdown()

	// Output:
	// Code runs before shutdown is called.
	// The action is performed after shutdown is called.
}

// ExampleUseOrder_firstInFirstDone demonstrates the "first in, first done"
// order.
func ExampleUseOrder_firstInFirstDone() {
	sa := safedownwe.NewShutdownActions(
		safedownwe.UseOrder(safedownwe.FirstInFirstDone),
	)

	sa.AddActions(func() {
		fmt.Println("The first action added will be done first ...")
	})
	sa.AddActions(func() {
		fmt.Println("... and the last action added will be done last.")
	})

	sa.Shutdown()

	// Output:
	// The first action added will be done first ...
	// ... and the last action added will be done last.
}

// ExampleUseOrder_firstInFirstDone demonstrates the "first in, last done"
// order.
func ExampleUseOrder_firstInLastDone() {
	sa := safedownwe.NewShutdownActions(
		safedownwe.UseOrder(safedownwe.FirstInLastDone),
	)

	sa.AddActions(func() {
		fmt.Println("... and the first action added will be done last.")
	})
	sa.AddActions(func() {
		fmt.Println("The last action added will be done first ...")
	})

	sa.Shutdown()

	// Output:
	// The last action added will be done first ...
	// ... and the first action added will be done last.
}

// ExampleUsePostShutdownStrategy demonstrates how to set a post shutdown strategy
// and its consequences.
func ExampleUsePostShutdownStrategy() {
	sa := safedownwe.NewShutdownActions(
		safedownwe.UsePostShutdownStrategy(safedownwe.PerformCoordinatelyInBackground),
	)

	sa.AddActions(func() {
		fmt.Println("... and the first action added will be done after that.")
	})
	sa.AddActions(func() {
		fmt.Println("The last action added will be done first ...")
	})

	sa.Shutdown()

	wg := sync.WaitGroup{}
	wg.Add(1)
	sa.AddActions(func() {
		fmt.Println("The action added after shutdown is also done (provided we wait a little).")
		wg.Done()
	})
	wg.Wait()

	// Output:
	// The last action added will be done first ...
	// ... and the first action added will be done after that.
	// The action added after shutdown is also done (provided we wait a little).
}

// endregion

// region Tests

// TestNewShutdownActions tests the NewShutdownActions constructor.
func TestNewShutdownActions(t *testing.T) {
	// Test that the constructor can be called without panicking.
	t.Run("no_panic", func(t *testing.T) {
		safedownwe.NewShutdownActions()
	})
}

// TestShutdownActions_AddActions tests the behaviour of the AddActions.
func TestShutdownActions_AddActions(t *testing.T) {
	// Testing that a single added action is performed on shutdown
	t.Run("single", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()
	})

	// Testing that multiple actions added in one call are performed
	// on shutdown.
	t.Run("multiple_inputs", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()

		sa.AddActions(
			createTestableShutdownAction(t, wg, counter, 3),
			createTestableShutdownAction(t, wg, counter, 2),
			createTestableShutdownAction(t, wg, counter, 1),
		)
		sa.Shutdown()
	})

	// Testing that actions added in multiple call are performed
	// on shutdown.
	t.Run("multiple calls", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()

		sa.AddActions(createTestableShutdownAction(t, wg, counter, 3))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 2))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()
	})
}

// TestShutdownActions_AddActionsWithErrors tests the behaviour of the AddActionsWithErrors.
func TestShutdownActions_AddActionsWithErrors(t *testing.T) {
	// Testing that a single added action is performed on shutdown
	t.Run("single", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 1, fmt.Errorf("error")))
		sa.Shutdown()
	})

	// Testing that multiple actions added in one call are performed
	// on shutdown.
	t.Run("multiple_inputs", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()

		sa.AddActionsWithErrors(
			createTestableShutdownActionWithError(t, wg, counter, 3, fmt.Errorf("error")),
			createTestableShutdownActionWithError(t, wg, counter, 2, fmt.Errorf("error")),
			createTestableShutdownActionWithError(t, wg, counter, 1, fmt.Errorf("error")),
		)
		sa.Shutdown()
	})

	// Testing that actions added in multiple call are performed
	// on shutdown.
	t.Run("multiple calls", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()

		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 3, fmt.Errorf("error")))
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 2, fmt.Errorf("error")))
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 1, fmt.Errorf("error")))
		sa.Shutdown()
	})
}

// TestShutdownActions_Shutdown tests the behaviour of the shutdown method.
func TestShutdownActions_Shutdown(t *testing.T) {
	// Testing that all shutdown actions are performed.
	t.Run("completeness", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()

		sa.AddActions(createTestableShutdownAction(t, wg, counter, 3))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 2))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()
	})

	// Testing that the shutdown method is idempotent.
	t.Run("idempotency", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()
		sa.Shutdown()
	})
}

// TestShutdownActions_Wait tests the behaviour of the wait method.
func TestShutdownActions_Wait(t *testing.T) {
	// The test cases are not
	const minimumWaitDuration = 10 * time.Millisecond

	// Tests that the Wait method waits before a shutdown and not after one.
	t.Run("shutdown", func(t *testing.T) {
		sa := safedownwe.NewShutdownActions()
		assertMethodIsTemporarilyBlocking(t, sa.Wait, minimumWaitDuration, "wait function before shutdown")

		// The inclusion of the wait means that if wait still blocks after shutdown
		// then this test will run into a timeout.
		sa.Shutdown()
		sa.Wait()
	})

	// TestShutdownActions_Wait_withSignal tests that the wait method waits before
	// a signal and not after one.
	t.Run("shutdown_on_signal", func(t *testing.T) {
		sa := safedownwe.NewShutdownActions(safedownwe.ShutdownOnSignals(os.Interrupt))
		assertMethodIsTemporarilyBlocking(t, sa.Wait, minimumWaitDuration, "wait function before signal received")

		// The inclusion of the wait means that if wait still blocks after shutdown
		// then this test will run into a timeout.
		sendOSSignalToSelf(os.Interrupt)
		sa.Wait()
	})
}

// TestUseOrder tests the use of the safedownwe.ShutdownOnAnySignal option.
func TestShutdownOnAnySignal(t *testing.T) {
	// Tests that the shutdown actions can still be shut down manually.
	t.Run("shutdown", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(safedownwe.ShutdownOnAnySignal())
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()
	})

	// Tests that shutdown will be called when a signal is received.
	t.Run("signal", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(safedownwe.ShutdownOnAnySignal())
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sendOSSignalToSelf(os.Interrupt)
	})

	// Tests that multiple shutdown actions can be initialised listing for the same
	// signal and both of them shutdown.
	t.Run("multiple_actions", func(t *testing.T) {
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		counter1 := new(int32)
		sa1 := safedownwe.NewShutdownActions(safedownwe.ShutdownOnAnySignal())
		sa1.AddActions(createTestableShutdownAction(t, wg, counter1, 1))

		counter2 := new(int32)
		sa2 := safedownwe.NewShutdownActions(safedownwe.ShutdownOnAnySignal())
		sa2.AddActions(createTestableShutdownAction(t, wg, counter2, 1))

		sendOSSignalToSelf(os.Interrupt)
	})
}

// TestUseOrder tests the use of the safedownwe.ShutdownOnSignals option.
func TestShutdownOnSignals(t *testing.T) {
	// Tests that the shutdown actions can still be shut down manually.
	t.Run("shutdown", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(safedownwe.ShutdownOnSignals(os.Interrupt))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()
	})

	// Tests that shutdown will be called when a signal is received.
	t.Run("signal", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(safedownwe.ShutdownOnSignals(os.Interrupt))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sendOSSignalToSelf(os.Interrupt)
	})

	// Tests that multiple shutdown actions can be initialised listing for different
	// signals and only one of them shutdown.
	t.Run("multiple_shutdown_actions_listening_for_different_signals", func(t *testing.T) {
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		counter1 := new(int32)
		sa1 := safedownwe.NewShutdownActions(safedownwe.ShutdownOnSignals(os.Interrupt))
		sa1.AddActions(createTestableShutdownAction(t, wg, counter1, 1))

		counter2 := new(int32)
		sa2 := safedownwe.NewShutdownActions(safedownwe.ShutdownOnSignals(os.Kill))
		sa2.AddActions(createTestableShutdownAction(t, wg, counter2, -1)) // This action must never be called

		sendOSSignalToSelf(os.Interrupt)

		// The extra call to Done are required because `sa2` will never be triggered.
		wg.Done()
	})

	// Tests that multiple shutdown actions can be initialised listing for the same
	// signal and both of them shutdown.
	t.Run("multiple_shutdown_actions_listening_for_same_signal", func(t *testing.T) {
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		counter1 := new(int32)
		sa1 := safedownwe.NewShutdownActions(safedownwe.ShutdownOnSignals(os.Interrupt))
		sa1.AddActions(createTestableShutdownAction(t, wg, counter1, 1))

		counter2 := new(int32)
		sa2 := safedownwe.NewShutdownActions(safedownwe.ShutdownOnSignals(os.Interrupt))
		sa2.AddActions(createTestableShutdownAction(t, wg, counter2, 1))

		sendOSSignalToSelf(os.Interrupt)
	})
}

// TestUseErrorChan tests the use of the safedownwe.TestUseErrorChan option.
func TestUseErrorChan(t *testing.T) {
	// Tests that if a single error occurs it is sent to the
	// error channel.
	t.Run("single_error", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		ch := make(chan error, 1)
		sa := safedownwe.NewShutdownActions(
			safedownwe.UseErrorChan(ch, false),
		)

		err := fmt.Errorf("error")
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 1, err))
		sa.Shutdown()

		assertErrorsInChan(t, ch, err)
	})

	// Tests that if a multiples error occurs, they are sent to the
	// error channel in the expected order.
	t.Run("multiple_errors", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		ch := make(chan error, 2)
		sa := safedownwe.NewShutdownActions(
			safedownwe.UseErrorChan(ch, false),
		)

		err1 := fmt.Errorf("error 1")
		err2 := fmt.Errorf("error 2")

		sa.AddActions(createTestableShutdownAction(t, wg, counter, 4))
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 3, nil))
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 2, err2))
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 1, err1))
		sa.Shutdown()

		assertErrorsInChan(t, ch, err1, err2)
	})

	t.Run("post_shutdown_error", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		ch := make(chan error, 1)
		sa := safedownwe.NewShutdownActions(
			safedownwe.UseErrorChan(ch, false),
			safedownwe.UsePostShutdownStrategy(safedownwe.PerformImmediately),
		)
		sa.Shutdown()

		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 1, fmt.Errorf("error")))
		assertErrorsInChan(t, ch)
	})

	// Tests that the shutdown is blocked by the channel if
	// there are too many errors relative to the capacity.
	t.Run("channel_blocks", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(3*time.Second))

		ch := make(chan error, 1)
		sa := safedownwe.NewShutdownActions(
			safedownwe.UseErrorChan(ch, false),
		)

		err1 := fmt.Errorf("error 1")
		err2 := fmt.Errorf("error 2")
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 2, err2))
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 1, err1))

		assertMethodIsTemporarilyBlocking(t, sa.Shutdown, time.Second, "shutdown must block when insufficient channel capacity")
		assertErrorsInChan(t, ch, err1, err2)
	})

	// Tests that the channel closes.
	t.Run("channel_closes", func(t *testing.T) {
		ch := make(chan error)
		sa := safedownwe.NewShutdownActions(
			safedownwe.UseErrorChan(ch, false),
		)
		sa.Shutdown()
		assertErrorsInChan(t, ch)
	})

	// Tests that the shutdown is blocked by the channel if
	// there are too many errors relative to the capacity.
	t.Run("discard_overflow", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(3*time.Second))

		ch := make(chan error, 2)
		sa := safedownwe.NewShutdownActions(
			safedownwe.UseErrorChan(ch, true),
		)

		err1 := fmt.Errorf("error 1")
		err2 := fmt.Errorf("error 2")
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 3, fmt.Errorf("discarded error")))
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 2, err2))
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 1, err1))
		sa.Shutdown()

		assertErrorsInChan(t, ch, err1, err2)
	})

	// Tests that if a nil channel is used errors are discarded
	t.Run("nil_channel", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(
			safedownwe.UseErrorChan(nil, false),
		)

		err := fmt.Errorf("error")
		sa.AddActionsWithErrors(createTestableShutdownActionWithError(t, wg, counter, 1, err))
		sa.Shutdown()
	})
}

// TestUseOrder tests the use of the safedownwe.UseOnSignalFunc option.
func TestUseOnSignalFunc(t *testing.T) {
	// Tests that the function passed in the UseOnSignalFunc does nothing
	// if shutdown is called.
	t.Run("shutdown", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(
			safedownwe.ShutdownOnAnySignal(),
			safedownwe.UseOnSignalFunc(func(signal os.Signal) {
				t.Logf("unexpected signal received: %v", signal)
				t.FailNow()
			}),
		)

		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()
	})

	// Tests that the function passed in the UseOnSignalFunc is called if a
	// signal is sent.
	t.Run("signal", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(
			safedownwe.ShutdownOnAnySignal(),
			safedownwe.UseOnSignalFunc(createTestableOnSignalFunction(t, wg, os.Interrupt)),
		)

		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sendOSSignalToSelf(os.Interrupt)
	})
}

// TestUseOrder tests the use of the safedownwe.UseOrder option.
func TestUseOrder(t *testing.T) {
	// Tests that all shutdown actions are performed in order when
	// safedownwe.UseOrder() is not used.
	t.Run("unused", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()

		sa.AddActions(createTestableShutdownAction(t, wg, counter, 3))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 2))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()
	})

	// Tests that all shutdown actions are performed in order when using:
	// safedownwe.UseOrder(safedownwe.FirstInLastDone).
	t.Run("first_in_last_done", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(
			safedownwe.UseOrder(safedownwe.FirstInLastDone),
		)

		sa.AddActions(createTestableShutdownAction(t, wg, counter, 3))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 2))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()
	})

	// Tests that all shutdown actions are performed in order when using:
	// safedownwe.UseOrder(safedownwe.FirstInFirstDone).
	t.Run("first_in_first_down", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(
			safedownwe.UseOrder(safedownwe.FirstInFirstDone),
		)

		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 2))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 3))
		sa.Shutdown()
	})

	// Tests that  if an invalid order is used the option will panic.
	t.Run("invalid_order", func(t *testing.T) {
		defer func() {
			var panicked bool
			if r := recover(); r != nil {
				panicked = true
			}

			if !panicked {
				t.Log("safedownwe.UseOrder was expected to panic")
				t.Fail()
			}
		}()

		safedownwe.UseOrder(42)
	})
}

// TestUseOrder tests the use of the safedownwe.UsePostShutdownStrategy option.
func TestUsePostShutdownStrategy(t *testing.T) {
	// Tests that no actions will be performed after shutdown has been called
	// when UsePostShutdownStrategy is not used.
	t.Run("unused", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions()
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()

		sa.AddActions(createTestableShutdownAction(t, wg, counter, -1))
		wg.Done()
	})

	// Tests that no actions will be performed after shutdown has been called
	// when using safedownwe.UsePostShutdownStrategy(safedownwe.DoNothing).
	t.Run("do_nothing", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(safedownwe.UsePostShutdownStrategy(safedownwe.DoNothing))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()

		sa.AddActions(createTestableShutdownAction(t, wg, counter, -1))
		wg.Done()
	})

	// Tests that actions can be performed after shutdown has been called in a way that
	// matches the PerformCoordinatelyInBackground description.
	t.Run("perform_coordinately_in_background", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(safedownwe.UsePostShutdownStrategy(safedownwe.PerformCoordinatelyInBackground))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 2))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()

		// The first action added will start the processing actions and will be the
		// first to be started. However, due to the delay the other two actions
		// will be added to a wait list. Due to the order the last will of the two
		// will be done first.

		sa.AddActions(createTestableShutdownActionWithDelay(t, wg, counter, 3, 5*time.Millisecond))
		time.Sleep(time.Millisecond)
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 5))
		time.Sleep(time.Millisecond)
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 4))
		time.Sleep(time.Millisecond)
	})

	// Tests that actions can be performed after shutdown has been called in a way that
	// matches the PerformImmediately description.
	t.Run("perform_immediately", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(safedownwe.UsePostShutdownStrategy(safedownwe.PerformImmediately))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 2))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()

		sa.AddActions(createTestableShutdownActionWithDelay(t, wg, counter, 3, 5*time.Millisecond))
		sa.AddActions(
			createTestableShutdownAction(t, wg, counter, 5),
			createTestableShutdownAction(t, wg, counter, 4),
		)
	})

	// Tests that actions can be performed after shutdown has been called in a way that
	// matches the PerformImmediatelyInBackground description.
	t.Run("perform_immediately_in_background", func(t *testing.T) {
		counter := new(int32)
		wg := new(sync.WaitGroup)
		defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

		sa := safedownwe.NewShutdownActions(safedownwe.UsePostShutdownStrategy(safedownwe.PerformImmediatelyInBackground))

		sa.AddActions(createTestableShutdownAction(t, wg, counter, 2))
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 1))
		sa.Shutdown()

		// All actions will start immediately in a go routine. It is a race
		// condition to determine which will increment the counter first. Due to the
		// delays/sleeps we obtain the expected values.

		sa.AddActions(createTestableShutdownActionWithDelay(t, wg, counter, 6, 5*time.Millisecond))
		time.Sleep(time.Millisecond)
		sa.AddActions(createTestableShutdownAction(t, wg, counter, 3))
		time.Sleep(time.Millisecond)
		sa.AddActions(
			createTestableShutdownAction(t, wg, counter, 5),
			createTestableShutdownAction(t, wg, counter, 4),
		)
		time.Sleep(time.Millisecond)
	})

	// Tests that if an invalid strategy is used the option will panic.
	t.Run("invalid_strategy", func(t *testing.T) {
		defer func() {
			var panicked bool
			if r := recover(); r != nil {
				panicked = true
			}

			if !panicked {
				t.Log("safedownwe.UsePostShutdownStrategy was expected to panic")
				t.Fail()
			}
		}()

		safedownwe.UsePostShutdownStrategy(42)
	})
}

// assertCounterValue fails the test if the value stored in the counter does
// not match the expected value.
func assertCounterValue(t *testing.T, counter *int32, expectedValue int32, scenario string) {
	actualValue := atomic.LoadInt32(counter)
	if actualValue == expectedValue {
		return
	}

	t.Logf("%s: mismatch between expected value (%d) and actual value (%d)", scenario, expectedValue, actualValue)
	t.FailNow()
}

// assertErrorsInChan fails the test if the err channel does not contain
// the given errors (in the given order).
func assertErrorsInChan(t *testing.T, errCh <-chan error, errs ...error) {
	var count int
	for {
		actualErr, ok := <-errCh
		if !ok {
			if count != len(errs) {
				t.Logf("mismatch between expected number of errors (%d) and actual number (%d)", len(errs), count)
				t.Fail()
			}
			return
		}

		if count >= len(errs) {
			t.Log("more errors in channel than expected")
			t.Fail()
			return
		}

		if expectedErr := errs[count]; !errors.Is(actualErr, expectedErr) {
			t.Logf("mismatch between expected error (%v) and actual error (%v) in position %d", expectedErr, actualErr, count)
			t.Fail()
		}

		count++
	}
}

// assertMethodIsTemporarilyBlocking checks that the method provided blocks for
// the duration provided.
//
// This is useful for checking methods that are expected to temporarily block.
// The assertion utilises concurrency and may give false results on occasion.
// The duration should be sufficient to take into account the starting of a
// goroutine. It is only intended for very simple blocking methods e.g. ones
// that are solely waiting on a chan to be closed.
func assertMethodIsTemporarilyBlocking(t *testing.T, method func(), duration time.Duration, scenario string) {
	var state int32
	go func() {
		method()
		atomic.StoreInt32(&state, 1)
	}()

	time.Sleep(duration)
	if !atomic.CompareAndSwapInt32(&state, 0, 1) {
		t.Logf("%s: method failed to block for duration", scenario)
		t.FailNow()
	}
}

func assertSignalEquality(t *testing.T, actual, expected os.Signal) {
	if actual == expected {
		return
	}

	t.Logf("mismatch between expected signal (%d) and actual signal (%d) received", expected, actual)
	t.FailNow()
}

// assertWaitGroupDoneBeforeDeadline is a way to quickly check that a test
// does not wait for a long time.
func assertWaitGroupDoneBeforeDeadline(t *testing.T, wg *sync.WaitGroup, deadline time.Time) {
	/*
		There is an accepted proposal in which each test can have its own
		individual timeout (https://github.com/golang/go/issues/48157). Once
		this has been implemented then this function can be removed.
	*/

	success := make(chan struct{})
	go func() {
		wg.Wait()
		close(success)
	}()

	// A context was chosen over a ticker in case the deadline was in the past.
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	select {
	case <-success:
	case <-ctx.Done():
		t.Logf("wait group was not done before deadline")
		t.FailNow()
	}
}

// createTestableShutdownAction creates an action to be used in tests. The
// counter is included to ensure that the actions occur the in the correct
// order.
func createTestableShutdownAction(t *testing.T, wg *sync.WaitGroup, counter *int32, expectedValue int32) func() {
	wg.Add(1)
	return func() {
		atomic.AddInt32(counter, 1)
		assertCounterValue(t, counter, expectedValue, "the counter in testable action encountered an issue")
		wg.Done()
	}
}

// createTestableShutdownActionWithError creates an action that returns an error
// to be used in tests. The counter is included to ensure that the actions occur
// the in the correct order.
func createTestableShutdownActionWithError(t *testing.T, wg *sync.WaitGroup, counter *int32, expectedValue int32, err error) func() error {
	wg.Add(1)
	return func() error {
		atomic.AddInt32(counter, 1)
		assertCounterValue(t, counter, expectedValue, "the counter in testable action encountered an issue")
		wg.Done()
		return err
	}
}

// createTestableShutdownActionWithDelay creates a testable action
// using createTestableShutdownAction but adds a delay before the action is
// performed.
//
// This is useful when for testing behaviour that happens asynchronously.
// Consequently, it is unreliable and is expected to sometimes fail.
func createTestableShutdownActionWithDelay(t *testing.T, wg *sync.WaitGroup, counter *int32, expectedValue int32, delay time.Duration) func() {
	action := createTestableShutdownAction(t, wg, counter, expectedValue)
	return func() {
		time.Sleep(delay)
		action()
	}
}

func createTestableOnSignalFunction(t *testing.T, wg *sync.WaitGroup, expectedSignal os.Signal) func(os.Signal) {
	wg.Add(1)
	return func(signal os.Signal) {
		assertSignalEquality(t, signal, expectedSignal)
		wg.Done()
	}
}

func sendOSSignalToSelf(signal os.Signal) {
	process := os.Process{Pid: os.Getpid()}
	if err := process.Signal(signal); err != nil {
		panic(fmt.Sprintf("test failed: unable to send signal (%v) to self", signal))
	}
}

// endregion
