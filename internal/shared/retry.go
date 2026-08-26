package shared

import (
	"context"
	"time"
)

var RetryDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

func DoWithRetries(
	ctx context.Context,
	delays []time.Duration,
	retriable func(error) bool,
	op func(ctx context.Context) error,
) error {
	err := op(ctx)
	for i := 0; err != nil && retriable(err) && i < len(delays); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delays[i]):
		}

		err = op(ctx)
	}

	return err
}
