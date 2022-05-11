package safedown_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PeterEFinch/safedown"
)

// region Tests

// TestShutdownActions_Shutdown tests that all shutdown actions are performed
// in order.
func TestShutdownActions_Shutdown(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions()
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 2))
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 3))
	sa.Shutdown()
}

func TestShutdownActions_Shutdown_idempotent(t *testing.T) {
	var counter int32
	wg := &sync.WaitGroup{}
	defer assertWaitGroupDoneBeforeDeadline(t, wg, time.Now().Add(time.Second))

	sa := safedown.NewShutdownActions()
	sa.AddActions(createTestableShutdownAction(t, wg, &counter, 1))
	sa.Shutdown()
	sa.Shutdown()
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

// endregion
