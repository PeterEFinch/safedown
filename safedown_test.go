package safedown_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PeterEFinch/safedown"
)

// region Examples

// Example_withSignalReceived demonstrates how setting up the safedown's
// shutdown actions works when a signal is received.
func Example_withSignalReceived() {
	// This will send an interrupt signal after a second to simulate a signal
	// being sent from the outside.
	go func(pid int) {
		time.Sleep(time.Second)
		process := os.Process{Pid: pid}
		if err := process.Signal(os.Interrupt); err != nil {
			panic("unable to continue test: could not send signal to process")
		}
	}(os.Getpid())

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	defer sa.Shutdown()
	sa.SetOnSignal(func(signal os.Signal) {
		fmt.Printf("Signal received: %s\n", signal.String())
	})

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

// Example_withoutSignalReceived demonstrates how setting up the safedown's
// shutdown actions works when no signal is received (and the program can
// terminate of its own accord).
func Example_withoutSignalReceived() {
	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	defer sa.Shutdown()
	sa.SetOnSignal(func(signal os.Signal) {
		fmt.Printf("Signal received: %s\n", signal.String())
	})

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

// Example_shutdown_firstInFirstDone demonstrates the "first in, first done"
// order.
func Example_shutdown_firstInFirstDone() {
	sa := safedown.NewShutdownActions(safedown.FirstInFirstDone)

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

// Example_shutdown_firstInLastDone demonstrates the "first in, last done"
// order.
func Example_shutdown_firstInLastDone() {
	sa := safedown.NewShutdownActions(safedown.FirstInLastDone)

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

// endregion

// region Tests

// TestShutdownActions_Shutdown tests that all shutdown actions are performed
// in order: first in, first done.
func TestShutdownActions_Shutdown_FirstInFirstDone(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInFirstDone)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 2))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 3))
	sa.Shutdown()
}

// TestShutdownActions_Shutdown tests that all shutdown actions are performed
// in order: first in, last done.
func TestShutdownActions_Shutdown_FirstInLastDone(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 3))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 2))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()
}

// TestShutdownActions_Shutdown_idempotent tests that the method Shutdown
// is idempotent.
func TestShutdownActions_Shutdown_idempotent(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()
	sa.Shutdown()
}

// TestShutdownActions_Shutdown_postShutdownAction tests any action after the
// shutdown actions have been triggered will not be performed.
func TestShutdownActions_Shutdown_postShutdownAction(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInFirstDone)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()

	sa.AddActions(createTestableShutdownAction(t, wg, &counter, -1))
	wg.Done()
}

// TestShutdownActions_Shutdown_withListening tests that the shutdown actions
// can still be shut down manually while listening for signals.
func TestShutdownActions_Shutdown_withListening(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()
}

// TestShutdownActions_SetPostShutdownStrategy_None tests that no actions will
// be performed after shutdown has been called.
func TestShutdownActions_SetPostShutdownStrategy_None(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa.SetPostShutdownStrategy(safedown.DoNothing) // This is the default strategy
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 2))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()

	sa.AddActions(createTestableShutdownAction(t, wg, &counter, -1))
	wg.Done()
}

// TestShutdownActions_SetPostShutdownStrategy_PerformCoordinately tests
// that actions can be performed after shutdown has been called in a way that
// matches the PerformCoordinately description.
func TestShutdownActions_SetPostShutdownStrategy_PerformCoordinately(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa.SetPostShutdownStrategy(safedown.PerformCoordinately)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 2))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()

	// The first action added will start the processing actions and will be the
	// first to be started. However, due to the delay the other two actions
	// will be added to a wait list. Due to the order the last will of the two
	// will be done first.

	sa.AddActions(createTestableShutdownActionWithDelay(t, wg, &counter, 3, 5*time.Millisecond))
	time.Sleep(time.Millisecond)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 5))
	time.Sleep(time.Millisecond)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 4))
	time.Sleep(time.Millisecond)
}

// TestShutdownActions_SetPostShutdownStrategy_PerformCoordinately tests
// that actions can be performed after shutdown has been called in a way that
// matches the PerformImmediately description.
func TestShutdownActions_SetPostShutdownStrategy_PerformImmediately(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa.SetPostShutdownStrategy(safedown.PerformImmediately)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 2))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()

	// All actions will start immediately in a go routine. It is a race
	// condition to determine which will increment the counter first. Due to the
	// delays/sleeps we obtain the expected values.

	sa.AddActions(createTestableShutdownActionWithDelay(t, wg, &counter, 5, 5*time.Millisecond))
	time.Sleep(time.Millisecond)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 3))
	time.Sleep(time.Millisecond)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 4))
	time.Sleep(time.Millisecond)

}

// TestShutdownActions_Wait_withShutdown tests that the wait method waits before
// a shutdown and not after one.
func TestShutdownActions_Wait_withShutdown(t *testing.T) {
	sa := safedown.NewShutdownActions(safedown.FirstInLastDone)
	assertMethodIsTemporarilyBlocking(t, sa.Wait, 10*time.Millisecond, "wait function before shutdown")

	// The inclusion of the wait means that if wait still blocks after shutdown
	// then this test will run into a timeout.
	sa.Shutdown()
	sa.Wait()
}

// TestShutdownActions_Wait_withSignal tests that the wait method waits before
// a signal and not after one.
func TestShutdownActions_Wait_withSignal(t *testing.T) {
	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	assertMethodIsTemporarilyBlocking(t, sa.Wait, 10*time.Millisecond, "wait function before signal received")

	// The inclusion of the wait means that if wait still blocks after shutdown
	// then this test will run into a timeout.
	sendOSSignalToSelf(os.Interrupt)
	sa.Wait()
}

// TestShutdownActions_signalReceived tests that shutdown will be called when
// a signal is received.
func TestShutdownActions_signalReceived(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 3))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 2))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sendOSSignalToSelf(os.Interrupt)
}

// TestShutdownActions_signalReceived_withOnSignal tests that onSignal method
// and shutdown will be called when a signal is received.
func TestShutdownActions_signalReceived_withOnSignal(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa.SetOnSignal(createTestableOnSignalFunction(t, wg, os.Interrupt))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 3))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 2))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sendOSSignalToSelf(os.Interrupt)
}

// TestShutdownActions_multiShutdownActions tests that multiple shutdown actions
// can be initialised with and shutdown while listen for the same signal.
func TestShutdownActions_multiShutdownActions(t *testing.T) {
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	var counter1 int32
	sa1 := safedown.NewShutdownActions(safedown.FirstInFirstDone, os.Interrupt)
	sa1.SetOnSignal(createTestableOnSignalFunction(t, wg, os.Interrupt))
	sa1.AddActions(createTestableShutdownAction(t, wg, &counter1, 1))

	var counter2 int32
	sa2 := safedown.NewShutdownActions(safedown.FirstInFirstDone, os.Interrupt)
	sa2.SetOnSignal(createTestableOnSignalFunction(t, wg, os.Interrupt))
	sa2.AddActions(createTestableShutdownAction(t, wg, &counter2, 1))

	sendOSSignalToSelf(os.Interrupt)
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
