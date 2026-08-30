package shared

import (
	"context"
	"time"
)

// RetryDelays — интервалы между дополнительными попытками: 1с, 3с, 5с.
// Итого не более четырёх попыток: первая + три повтора.
var RetryDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

// DoWithRetries выполняет op и повторяет её с растущими интервалами из delays,
// пока retriable возвращает true для ошибки. Возвращает последнюю ошибку
// или первый успешный результат.
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
