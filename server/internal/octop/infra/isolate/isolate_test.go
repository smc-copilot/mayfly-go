package isolate

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunIsolatesPartialFailure(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8}
	var okCount atomic.Int32
	res := Run(context.Background(), items, 3, time.Second, func(ctx context.Context, n int) error {
		if n == 3 {
			return errors.New("instance 3 down")
		}
		if n == 7 {
			time.Sleep(10 * time.Millisecond)
			return errors.New("instance 7 timeout-ish")
		}
		okCount.Add(1)
		return nil
	})
	require.Len(t, res, 8)
	require.Equal(t, int32(6), okCount.Load())
	require.Error(t, res[2].Err)
	require.Error(t, res[6].Err)
	require.NoError(t, res[0].Err)
	require.NoError(t, res[7].Err)
}

func TestRunTimeoutDoesNotCancelSiblings(t *testing.T) {
	items := []int{1, 2}
	res := Run(context.Background(), items, 2, 30*time.Millisecond, func(ctx context.Context, n int) error {
		if n == 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return nil
			}
		}
		return nil
	})
	require.Error(t, res[0].Err)
	require.NoError(t, res[1].Err)
}
