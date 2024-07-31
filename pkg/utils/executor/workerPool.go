package executor

import (
	"runtime"
	"sync"
	atomic2 "sync/atomic"
)

// Task represents a unit of work to be processed by the worker pool
type Task struct {
	id  int
	fut *IFuture
}

var taskIds atomic2.Int64

// NewTask initializes a new Task with a unique ID and a payload function
func NewTask(fut *IFuture) Task {
	return Task{
		id:  int(taskIds.Add(1)),
		fut: fut,
	}
}

// WorkerPool is a struct that manages a pool of workers to process tasks
type WorkerPool struct {
	taskQueue   chan Task
	workerQueue chan struct{}
	wg          sync.WaitGroup
}

// NewWorkerPool initializes a new WorkerPool with the optimal number of workers
func NewWorkerPool() *WorkerPool {
	maxWorkers := runtime.NumCPU() * 2 // Allow more concurrency
	pool := &WorkerPool{
		taskQueue:   make(chan Task),
		workerQueue: make(chan struct{}, maxWorkers), // Buffered channel to limit max concurrent workers
	}
	go pool.dispatch()
	return pool
}

// NewWorkerPool initializes a new WorkerPool with the optimal number of workers
func NewWorkerPoolWithMax(maxWorkers int) *WorkerPool {
	pool := &WorkerPool{
		taskQueue:   make(chan Task),
		workerQueue: make(chan struct{}, maxWorkers), // Buffered channel to limit max concurrent workers
	}
	go pool.dispatch()
	return pool
}

// Start initializes the pool to start listening for tasks and dynamically start workers
func (wp *WorkerPool) dispatch() {
	for task := range wp.taskQueue {
		wp.wg.Add(1)
		select {
		case wp.workerQueue <- struct{}{}:
			go wp.worker(task)
		default:
			go wp.worker(task)
		}
	}
}

// worker processes a single task
func (wp *WorkerPool) worker(task Task) {
	defer wp.wg.Done()
	defer func() { <-wp.workerQueue }()
	task.fut.runInThisThread()
}

// SubmitWithError adds a task to the task queue to be processed by the workers and logs error if there was one
func SubmitWithError[T any](wp *WorkerPool, defaultValue T, task func() (T, error)) *Future[T] {
	fut := NewIFuture(task, wp)
	wp.submitFuture(fut)
	return NewFuture[T](fut, defaultValue)
}

// Execute adds a task to the task queue to be processed by the workers
func Execute[T any](wp *WorkerPool, task func()) {
	wp.execute(task)
}

// execute adds a task to the task queue to be processed by the workers
func (wp *WorkerPool) execute(task func()) {
	wp.submitFuture(NewIFuture[interface{}](func() (interface{}, error) {
		task()
		return nil, nil
	}, wp))
}

func Submit[T any](wp *WorkerPool, defaultValue T, task func()) *Future[T] {
	return NewFuture(wp.submit(task), defaultValue)
}

// submit adds a task to the task queue to be processed by the workers
func (wp *WorkerPool) submit(task func()) *IFuture {
	fut := NewIFuture[interface{}](func() (interface{}, error) {
		task()
		return nil, nil
	}, wp)
	wp.submitFuture(fut)
	return fut
}

// submitFuture adds a task to the task queue to be processed by the workers
func (wp *WorkerPool) submitFuture(fut *IFuture) {
	wp.taskQueue <- NewTask(fut)
}

// Stop gracefully shuts down the worker pool by closing the task queue and waiting for workers to finish
func (wp *WorkerPool) Stop() {
	close(wp.taskQueue)
	wp.wg.Wait()
}

func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}
