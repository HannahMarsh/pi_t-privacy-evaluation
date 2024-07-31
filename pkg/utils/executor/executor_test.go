package executor

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestWorkerPool_SubmitWithError(t *testing.T) {
	pool := NewWorkerPool()
	defer pool.Stop()

	future := SubmitWithError(pool, 0, func() (int, error) {
		time.Sleep(100 * time.Millisecond)
		return 42, nil
	})

	result, err := future.Get()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if result != 42 {
		t.Fatalf("Expected 42, got %v", result)
	}
}

func TestWorkerPool_SubmitWithError_QueueFull(t *testing.T) {
	pool := NewWorkerPool()
	defer pool.Stop()

	// Fill up the taskQueue to make it full
	for i := 0; i < cap(pool.taskQueue); i++ {
		SubmitWithError(pool, 0, func() (int, error) {
			time.Sleep(1 * time.Second)
			return 42, nil
		})
	}

	// Submit a task when the queue is full
	start := time.Now()
	future := SubmitWithError(pool, 0, func() (int, error) {
		return 84, nil
	})
	duration := time.Since(start)

	result, err := future.Get()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if result != 84 {
		t.Fatalf("Expected 84, got %v", result)
	}
	if duration > 100*time.Millisecond {
		t.Fatalf("The task should run in the calling thread")
	}
}

func TestWorkerPool_Submit(t *testing.T) {
	pool := NewWorkerPool()
	defer pool.Stop()

	future := Submit(pool, false, func() {
		time.Sleep(100 * time.Millisecond)
	})

	result, err := future.Get()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if result != false {
		t.Fatalf("Expected false, got %v", result)
	}
}

func TestFuture_ThenApply(t *testing.T) {
	pool := NewWorkerPool()
	defer pool.Stop()

	future := SubmitWithError(pool, 0, func() (int, error) {
		time.Sleep(100 * time.Millisecond)
		return 42, nil
	}).ThenApply(func(result int, err error) (int, error) {
		if err != nil {
			return 0, err
		}
		return result * 2, nil
	})

	result, err := future.Get()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if result != 84 {
		t.Fatalf("Expected 84, got %v", result)
	}
}

func TestFuture_HandleError(t *testing.T) {
	pool := NewWorkerPool()
	defer pool.Stop()

	future := SubmitWithError(pool, 0, func() (int, error) {
		return 0, fmt.Errorf("some error")
	})

	var caughtErr error

	var wg sync.WaitGroup
	wg.Add(1)

	future.HandleError(func(err error) {
		caughtErr = err
		wg.Done()
	})

	_, err := future.Get()
	wg.Wait()

	if caughtErr == nil || err == nil {
		t.Fatalf("Expected an error")
	}
	if caughtErr.Error() != "some error" || err.Error() != "some error" {
		t.Fatalf("Expected 'some error', got %v", caughtErr)
	}
}

func TestIFuture_Map(t *testing.T) {
	pool := NewWorkerPool()
	defer pool.Stop()

	future := SubmitWithError(pool, 0, func() (int, error) {
		return 42, nil
	}).Map(func(result int) (int, error) {
		return result * 2, nil
	}).Map(func(result int) (int, error) {
		return result * 2, nil
	})

	result, err := future.Get()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if result != 168 {
		t.Fatalf("Expected 84, got %v", result)
	}
}
