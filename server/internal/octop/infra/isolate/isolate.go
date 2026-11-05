package isolate

import (
	"context"
	"sync"
	"time"

	"mayfly-go/pkg/gox"
)

// Result 单实例执行结果。失败不影响其他实例。
type Result[T any] struct {
	Item  T
	Err   error
	Panic error
}

// Run 以有限并发、单任务超时执行 fn。某个任务失败或超时只影响自己。
func Run[T any](ctx context.Context, items []T, concurrency int, timeout time.Duration, fn func(context.Context, T) error) []Result[T] {
	if concurrency <= 0 {
		concurrency = 3
	}
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	out := make([]Result[T], len(items))
	if len(items) == 0 {
		return out
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, item := range items {
		i, item := i, item
		out[i].Item = item
		wg.Add(1)
		gox.Go(func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				out[i].Err = ctx.Err()
				return
			}
			defer func() { <-sem }()

			itemCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			var runErr error
			func() {
				defer gox.Recover(func(e error) {
					runErr = e
					out[i].Panic = e
				})
				runErr = fn(itemCtx, item)
			}()
			out[i].Err = runErr
		})
	}
	wg.Wait()
	return out
}
