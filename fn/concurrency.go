package fn

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ErrFunc is a type def for a function that takes a context (to allow early
// cancellation) and a series of value returning an error. This is typically
// used a closure to perform concurrent work over a homogeneous slice of
// values.
type ErrFunc[V any] func(context.Context, V) error

// ParSlice can be used to execute a function on each element of a slice in
// parallel. This function is fully blocking and will wait for all goroutines
// to either succeed, or for the first to error out.  Active goroutines limited
// with number of CPU.  Context will be passed in executable func and canceled
// the first time a function passed returns a non-nil error.  Returns the first
// non-nil error (if any).
func ParSlice[V any](ctx context.Context, s []V, f ErrFunc[V]) error {
	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(runtime.NumCPU())

	for _, v := range s {
		v := v
		errGroup.Go(func() error {
			return f(ctx, v)
		})
	}

	return errGroup.Wait()
}

// ParSliceErrCollect can be used to execute a function on each element of a
// slice in parallel. This function is fully blocking and will wait for all
// goroutines to finish. Error will be collected and returned as a map of slice
// element index to error. Active goroutines limited with number of CPU.
func ParSliceErrCollect[V any](ctx context.Context, s []V,
	f ErrFunc[V]) (map[int]error, error) {

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(runtime.NumCPU())

	var instanceErrorsMutex sync.Mutex
	instanceErrors := make(map[int]error, len(s))

	for idx, v := range s {
		v := v
		errGroup.Go(func() error {
			err := f(ctx, v)
			if err != nil {
				instanceErrorsMutex.Lock()
				instanceErrors[idx] = err
				instanceErrorsMutex.Unlock()
			}

			// Avoid returning an error here, as that would cancel
			// the errGroup and terminate all slice element
			// processing instances. Instead, collect the error and
			// return it later.
			return nil
		})
	}

	err := errGroup.Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to wait on error group in "+
			"ParSliceErrorCollect: %w", err)
	}

	return instanceErrors, nil
}
