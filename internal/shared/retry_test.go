package shared

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoWithRetries(t *testing.T) {
	fastDelays := []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}

	t.Run("succeeds on first attempt", func(t *testing.T) {
		attempts := 0
		err := DoWithRetries(context.Background(), fastDelays, func(error) bool { return true },
			func(context.Context) error {
				attempts++
				return nil
			})

		require.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("succeeds after two failures", func(t *testing.T) {
		attempts := 0
		err := DoWithRetries(context.Background(), fastDelays, func(error) bool { return true },
			func(context.Context) error {
				attempts++
				if attempts < 3 {
					return errors.New("temporary")
				}
				return nil
			})

		require.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("stops after three additional attempts", func(t *testing.T) {
		attempts := 0
		err := DoWithRetries(context.Background(), fastDelays, func(error) bool { return true },
			func(context.Context) error {
				attempts++
				return errors.New("always fails")
			})

		require.Error(t, err)
		assert.Equal(t, 4, attempts, "first attempt plus three retries")
	})

	t.Run("does not retry non-retriable error", func(t *testing.T) {
		attempts := 0
		err := DoWithRetries(context.Background(), fastDelays, func(error) bool { return false },
			func(context.Context) error {
				attempts++
				return errors.New("permanent")
			})

		require.Error(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("stops on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		attempts := 0
		err := DoWithRetries(ctx, []time.Duration{time.Second}, func(error) bool { return true },
			func(context.Context) error {
				attempts++
				return errors.New("temporary")
			})

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, attempts)
	})
}
