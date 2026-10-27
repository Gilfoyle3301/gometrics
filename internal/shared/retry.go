package shared

import (
	"context"
	"time"
)

// RetryDelays — задержки между повторными попытками операций с базой данных.
var RetryDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

// DoWithRetries выполняет op и при ошибке, которую retriable признаёт
// повторяемой, делает до len(delays) попыток с указанными паузами.
// Отмена ctx прерывает ожидание очередной попытки.
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
