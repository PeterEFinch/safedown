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

func TestShutdownActions_Shutdown_idempotent(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()
	sa.Shutdown()
}

func TestShutdownActions_Shutdown_withListening(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()
}

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

func TestShutdownActions_signalReceived_withOnSignal(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa.SetOnSignal(createTestableOnSignalAction(t, wg, os.Interrupt))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 3))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 2))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sendOSSignalToSelf(os.Interrupt)
}

func TestShutdownActions_multiShutdownActions(t *testing.T) {
	deadline := time.Now().Add(time.Second)

	var counter1 int32
	wg1 := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg1, deadline)
	sa1 := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa1.SetOnSignal(createTestableOnSignalAction(t, wg1, os.Interrupt))
	sa1.AddActions(createTestableShutdownAction(t, wg1, &counter1, 1))

	var counter2 int32
	wg2 := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg2, deadline)
	sa2 := safedown.NewShutdownActions(safedown.FirstInLastDone, os.Interrupt)
	sa2.SetOnSignal(createTestableOnSignalAction(t, wg2, os.Interrupt))
	sa2.AddActions(createTestableShutdownAction(t, wg2, &counter2, 1))

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

func createTestableOnSignalAction(t *testing.T, wg *sync.WaitGroup, expectedSignal os.Signal) func(os.Signal) {
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
