package worker

import (
	"sync"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	var mu sync.Mutex
	results := make([]int, 0)

	// Create a worker pool with 3 workers.
	pool := NewPool(3)

	// Submit some tasks to the pool.
	for i := range 10 {
		taskNum := i
		pool.Submit(func() {
			mu.Lock()
			results = append(results, taskNum)
			mu.Unlock()
		})
	}

	// Stop the worker pool and wait for all workers to finish.
	pool.Stop()

	// Check if all tasks were processed.
	if len(results) != 10 {
		t.Errorf("Expected 10 tasks to be processed, but got %d", len(results))
	}
}

func TestPoolRejectsSubmitAfterStop(t *testing.T) {
	pool := NewPool(1)
	pool.Stop()

	accepted := pool.Submit(func() {})
	if accepted {
		t.Fatalf("expected submit to be rejected after stop")
	}
}

func TestPoolUsesAtLeastOneWorker(t *testing.T) {
	pool := NewPool(0)
	defer pool.Stop()

	done := make(chan struct{})
	if !pool.Submit(func() {
		close(done)
	}) {
		t.Fatalf("expected submit to be accepted")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("task was not executed")
	}
}

func TestPoolRejectsSubmitAfterStopStarts(t *testing.T) {
	pool := NewPool(1)

	start := make(chan struct{})
	release := make(chan struct{})
	if !pool.Submit(func() {
		close(start)
		<-release
	}) {
		t.Fatalf("expected first task to be accepted")
	}
	<-start

	stopDone := make(chan struct{})
	go func() {
		pool.Stop()
		close(stopDone)
	}()

	deadline := time.After(2 * time.Second)
	for !pool.stopping.Load() {
		select {
		case <-deadline:
			t.Fatal("stop did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	accepted := pool.Submit(func() {})
	if accepted {
		t.Fatalf("expected submit to be rejected once stop starts")
	}

	close(release)

	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("stop did not complete")
	}
}

func TestPoolStopUnblocksBlockedSubmit(t *testing.T) {
	pool := NewPool(1)

	start := make(chan struct{})
	release := make(chan struct{})
	if !pool.Submit(func() {
		close(start)
		<-release
	}) {
		t.Fatalf("expected first task to be accepted")
	}
	<-start

	result := make(chan bool, 1)
	go func() {
		result <- pool.Submit(func() {})
	}()

	stopDone := make(chan struct{})
	go func() {
		pool.Stop()
		close(stopDone)
	}()

	close(release)

	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("stop did not complete")
	}

	select {
	case <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked submit did not return")
	}
}

func TestPoolStopDoesNotDeadlockWithBlockedSubmit(t *testing.T) {
	pool := NewPool(1)

	started := make(chan struct{})
	if !pool.Submit(func() {
		close(started)
		<-pool.quit
	}) {
		t.Fatalf("expected first task to be accepted")
	}
	<-started

	submitDone := make(chan bool, 1)
	go func() {
		submitDone <- pool.Submit(func() {})
	}()

	stopDone := make(chan struct{})
	go func() {
		pool.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("stop deadlocked with blocked submit")
	}

	select {
	case <-submitDone:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked submit did not return")
	}
}
