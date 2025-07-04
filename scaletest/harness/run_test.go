package harness_test

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coder/coder/v2/scaletest/harness"
)

// testFns implements Runnable and Cleanable.
type testFns struct {
	RunFn func(ctx context.Context, id string, logs io.Writer) error
	// CleanupFn is optional if no cleanup is required.
	CleanupFn func(ctx context.Context, id string, logs io.Writer) error
	// getBytesTransferred is optional if byte transfer tracking is required.
	getBytesTransferred func() (int64, int64)
}

// Run implements Runnable.
func (fns testFns) Run(ctx context.Context, id string, logs io.Writer) error {
	return fns.RunFn(ctx, id, logs)
}

// GetBytesTransferred implements Collectable.
func (fns testFns) GetBytesTransferred() (bytesRead int64, bytesWritten int64) {
	if fns.getBytesTransferred == nil {
		return 0, 0
	}

	return fns.getBytesTransferred()
}

// Cleanup implements Cleanable.
func (fns testFns) Cleanup(ctx context.Context, id string, logs io.Writer) error {
	if fns.CleanupFn == nil {
		return nil
	}

	return fns.CleanupFn(ctx, id, logs)
}

func Test_TestRun(t *testing.T) {
	t.Parallel()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		var (
			name, id          = "test", "1"
			runCalled         int64
			cleanupCalled     int64
			collectableCalled int64

			testFns = testFns{
				RunFn: func(ctx context.Context, id string, logs io.Writer) error {
					atomic.AddInt64(&runCalled, 1)
					return nil
				},
				CleanupFn: func(ctx context.Context, id string, logs io.Writer) error {
					atomic.AddInt64(&cleanupCalled, 1)
					return nil
				},
				getBytesTransferred: func() (int64, int64) {
					atomic.AddInt64(&collectableCalled, 1)
					return 0, 0
				},
			}
		)

		run := harness.NewTestRun(name, id, testFns)
		require.Equal(t, fmt.Sprintf("%s/%s", name, id), run.FullID())

		err := run.Run(context.Background())
		require.NoError(t, err)
		require.EqualValues(t, 1, atomic.LoadInt64(&runCalled))
		require.EqualValues(t, 1, atomic.LoadInt64(&collectableCalled))

		err = run.Cleanup(context.Background())
		require.NoError(t, err)
		require.EqualValues(t, 1, atomic.LoadInt64(&cleanupCalled))
	})

	t.Run("Cleanup", func(t *testing.T) {
		t.Parallel()

		t.Run("NoFn", func(t *testing.T) {
			t.Parallel()

			run := harness.NewTestRun("test", "1", testFns{
				RunFn: func(ctx context.Context, id string, logs io.Writer) error {
					return nil
				},
				CleanupFn: nil,
			})

			err := run.Cleanup(context.Background())
			require.NoError(t, err)
		})

		t.Run("NotDone", func(t *testing.T) {
			t.Parallel()

			var cleanupCalled int64
			run := harness.NewTestRun("test", "1", testFns{
				RunFn: func(ctx context.Context, id string, logs io.Writer) error {
					return nil
				},
				CleanupFn: func(ctx context.Context, id string, logs io.Writer) error {
					atomic.AddInt64(&cleanupCalled, 1)
					return nil
				},
			})

			err := run.Cleanup(context.Background())
			require.NoError(t, err)
			require.EqualValues(t, 0, atomic.LoadInt64(&cleanupCalled))
		})
	})

	t.Run("Collectable", func(t *testing.T) {
		t.Parallel()

		t.Run("NoFn", func(t *testing.T) {
			t.Parallel()

			run := harness.NewTestRun("test", "1", testFns{
				RunFn: func(ctx context.Context, id string, logs io.Writer) error {
					return nil
				},
				getBytesTransferred: nil,
			})

			err := run.Run(context.Background())
			require.NoError(t, err)
		})
	})

	t.Run("CatchesRunPanic", func(t *testing.T) {
		t.Parallel()

		testFns := testFns{
			RunFn: func(ctx context.Context, id string, logs io.Writer) error {
				panic(testPanicMessage)
			},
		}

		run := harness.NewTestRun("test", "1", testFns)

		err := run.Run(context.Background())
		require.Error(t, err)
		require.ErrorContains(t, err, "panic")
		require.ErrorContains(t, err, testPanicMessage)
	})

	t.Run("ResultPanicsWhenNotDone", func(t *testing.T) {
		t.Parallel()

		run := harness.NewTestRun("test", "1", testFns{})

		require.Panics(t, func() {
			_ = run.Result()
		})
	})
}
