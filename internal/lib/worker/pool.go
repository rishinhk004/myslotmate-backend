package worker

import (
	"context"
	"fmt"
	"sync"
)

// Task represents a unit of work
type Task func()

// WorkerPool implements the Executor pattern to manage concurrency
type WorkerPool struct {
	taskQueue  chan Task
	numWorkers int
	wg         sync.WaitGroup
	quit       chan struct{}
}

// NewWorkerPool Factory for creating a worker pool
func NewWorkerPool(numWorkers int, queueSize int) *WorkerPool {
	return &WorkerPool{
		taskQueue:  make(chan Task, queueSize),
		numWorkers: numWorkers,
		quit:       make(chan struct{}),
	}
}

// Start initializes the workers
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// worker function processes tasks from the queue
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	// fmt.Printf("Worker %d started\n", id)
	for {
		select {
		case task := <-wp.taskQueue:
			// Execute the task
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("Worker %d recovered from panic: %v\n", id, r)
					}
				}()
				task()
			}()
		case <-wp.quit:
			// fmt.Printf("Worker %d stopping\n", id)
			return
		}
	}
}

// Submit adds a task to the queue (Non-blocking or blocking depending on queue size)
func (wp *WorkerPool) Submit(task Task) {
	select {
	case wp.taskQueue <- task:
		// Task submitted
	default:
		// Queue is full, handle fallback or log error
		fmt.Println("WorkerPool queue is full, task dropped or handled synchronously")
		// Could execute synchronously as fallback or expanding the pool
		go task()
	}
}

// Stop gracefully shuts down the pool
func (wp *WorkerPool) Stop(ctx context.Context) {
	close(wp.quit)

	// Create a channel to signal wait group completion
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers finished
	case <-ctx.Done():
		// Context timeout
		fmt.Println("WorkerPool stop timed out")
	}
}
