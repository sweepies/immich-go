package worker

import (
	"sync"
	"sync/atomic"
)

// Task represents a unit of work to be processed by the worker pool.
type Task func()

// Pool manages a pool of worker goroutines.
type Pool struct {
	tasks    chan Task
	wg       sync.WaitGroup
	quit     chan struct{}
	stopOnce sync.Once
	stopping atomic.Bool
	submitMu sync.Mutex
}

// NewPool creates a new Pool with a specified number of workers.
func NewPool(numWorkers int) *Pool {
	if numWorkers <= 0 {
		numWorkers = 1
	}

	pool := &Pool{
		tasks: make(chan Task),
		quit:  make(chan struct{}),
	}

	for range numWorkers {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

// worker is the function that each worker goroutine runs.
func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case task := <-p.tasks:
			task()
		case <-p.quit:
			return
		}
	}
}

// Submit adds a task to the worker pool.
// It returns true when the task is accepted.
func (p *Pool) Submit(task Task) bool {
	return p.TrySubmit(task)
}

// TrySubmit adds a task to the worker pool.
// It returns false if the pool has started shutting down.
func (p *Pool) TrySubmit(task Task) bool {
	if p.stopping.Load() {
		return false
	}

	p.submitMu.Lock()
	defer p.submitMu.Unlock()

	if p.stopping.Load() {
		return false
	}

	select {
	case <-p.quit:
		return false
	case p.tasks <- task:
		return true
	}
}

// Stop stops all the workers and waits for them to finish.
func (p *Pool) Stop() {
	p.stopOnce.Do(func() {
		p.stopping.Store(true)

		p.submitMu.Lock()
		close(p.quit)
		p.submitMu.Unlock()

		p.wg.Wait()
	})
}
