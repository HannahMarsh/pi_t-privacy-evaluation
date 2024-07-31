package executor

import (
	pl "github.com/HannahMarsh/PrettyLogger"
	"sync"
)

type Future[T any] struct {
	iFut         *IFuture
	defaultValue T
}

func NewFuture[T any](iFut *IFuture, defaultValue T) *Future[T] {
	return &Future[T]{
		iFut:         iFut,
		defaultValue: defaultValue,
	}
}

func (f *Future[T]) CastOrDefault(result interface{}, err error) (T, error) {
	if err != nil || result == nil {
		return f.defaultValue, err
	} else {
		if value, ok := result.(T); !ok {
			return f.defaultValue, pl.NewError("failed to cast result to type T")
		} else {
			return value, nil
		}
	}
}

func (f *Future[T]) Get() (T, error) {
	result, err := f.iFut.Get()
	return f.CastOrDefault(result, err)
}

func (f *Future[T]) IsDone() bool {
	return f.iFut.IsDone()
}

func (f *Future[T]) IsRunning() bool {
	return f.iFut.IsRunning()
}

func (f *Future[T]) Map(next func(T) (T, error)) *Future[T] {
	return &Future[T]{
		iFut: f.iFut.Map(func(result interface{}) (interface{}, error) {
			if value, err := f.CastOrDefault(result, nil); err == nil {
				return next(value)
			} else {
				return value, err
			}
		}),
		defaultValue: f.defaultValue,
	}
}

func (f *Future[T]) HandleError(handleError func(error)) {
	f.ThenAccept(func(result T, err error) {
		if err != nil {
			handleError(err)
		}
	})
}

func (f *Future[T]) ThenAccept(next func(T, error)) {
	f.iFut.ThenAccept(func(result interface{}, err error) {
		next(f.CastOrDefault(result, err))
	})
}

func (f *Future[T]) ThenApply(next func(T, error) (T, error)) *Future[T] {
	return &Future[T]{
		iFut: f.iFut.ThenApply(func(result interface{}, err error) (interface{}, error) {
			return next(f.CastOrDefault(result, err))
		}),
		defaultValue: f.defaultValue,
	}
}

type IFuture struct {
	result       interface{}
	payload      func() (interface{}, error)
	err          error
	wg           sync.WaitGroup
	isDone       bool
	isRunning    bool
	workerPool   *WorkerPool
	mu           sync.RWMutex
	dependencies []*IFuture
}

func NewIFuture[T any](payload func() (T, error), pool *WorkerPool) *IFuture {
	f := &IFuture{
		payload: func() (interface{}, error) {
			i, err := payload()
			if err != nil {
				return nil, err
			}
			return (interface{})(i), nil
		},
		workerPool:   pool,
		dependencies: make([]*IFuture, 0),
	}
	f.wg.Add(1)
	return f
}

func (f *IFuture) runInThisThread() {
	if f.IsRunningOrDone() {
		return
	} else {
		f.mu.Lock()
		if f.isRunning || f.isDone {
			f.mu.Unlock()
			return
		}
		f.isRunning = true
		f.mu.Unlock()
	}

	if value, err := f.payload(); err != nil {
		f.completeWithError(err)
	} else {
		f.complete(value)
	}
}

func (f *IFuture) complete(result interface{}) {
	if f.IsDone() {
		return
	}
	f.mu.Lock()
	if f.isDone {
		f.mu.Unlock()
		return
	}
	f.result = result
	f.isDone = true
	f.wg.Done()
	dep := f.dependencies
	f.dependencies = make([]*IFuture, 0)
	f.mu.Unlock()

	for _, fut := range dep {
		f.workerPool.submitFuture(fut)
	}
}

func (f *IFuture) completeWithError(err error) {
	if f.IsDone() {
		return
	}
	f.mu.Lock()
	if f.isDone {
		f.mu.Unlock()
		return
	}
	f.err = err
	f.isDone = true
	f.wg.Done()
	dep := f.dependencies
	f.dependencies = make([]*IFuture, 0)
	f.mu.Unlock()

	for _, fut := range dep {
		f.workerPool.submitFuture(fut)
	}
}

func (f *IFuture) Get() (interface{}, error) {
	f.runInThisThread()
	f.wg.Wait()
	return f.result, f.err
}

func (f *IFuture) IsDone() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.isDone
}

func (f *IFuture) IsRunning() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.isRunning
}

func (f *IFuture) IsRunningOrDone() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.isRunning || f.isDone
}

func (f *IFuture) Map(next func(interface{}) (interface{}, error)) *IFuture {
	fut := NewIFuture[interface{}](func() (interface{}, error) {
		if result, err := f.Get(); err == nil {
			result2, err2 := next(result)
			return result2, err2
		} else {
			return nil, err
		}
	}, f.workerPool)
	f.mu.Lock()
	if !f.isDone {
		f.dependencies = append(f.dependencies, fut)
		f.mu.Unlock()
	} else {
		f.mu.Unlock()
		f.workerPool.submitFuture(fut)
	}
	return fut
}

func (f *IFuture) FlatMap(next func(interface{}) (interface{}, error)) interface{} {
	fut := f.Map(next)
	if result, err := fut.Get(); err != nil {
		return result
	} else {
		return nil
	}
}

func (f *IFuture) ThenAccept(next func(interface{}, error)) {
	fut := NewIFuture[interface{}](func() (interface{}, error) {
		result, err := f.Get()
		next(result, err)
		return nil, nil
	}, f.workerPool)
	f.addDependency(fut)
}

func (f *IFuture) ThenApply(next func(interface{}, error) (interface{}, error)) *IFuture {
	fut := NewIFuture[interface{}](func() (interface{}, error) {
		result, err := f.Get()
		return next(result, err)
	}, f.workerPool)
	f.addDependency(fut)
	return fut
}

func (f *IFuture) addDependency(fut *IFuture) {
	f.mu.Lock()
	if !f.isDone {
		f.dependencies = append(f.dependencies, fut)
		f.mu.Unlock()
	} else {
		f.mu.Unlock()
		f.workerPool.submitFuture(fut)
	}
}
