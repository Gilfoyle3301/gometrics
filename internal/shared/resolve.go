package shared

import (
	"fmt"
	"strconv"
	"time"
)

// Resolve выбирает значение параметра конфигурации по приоритету:
// явно установленный флаг командной строки, затем переменная окружения,
// затем JSON-файл конфигурации и в конце — значение флага по умолчанию.
// nil-указатели envVal и fileVal означают, что источник значение не задал.
func Resolve[T any](flagSet bool, flagVal T, envVal, fileVal *T) T {
	switch {
	case flagSet:
		return flagVal
	case envVal != nil:
		return *envVal
	case fileVal != nil:
		return *fileVal
	default:
		return flagVal
	}
}

// ParseIntervalSeconds разбирает интервал из JSON-файла конфигурации:
// строку-длительность ("1s", "2m") или целое число секунд ("30").
func ParseIntervalSeconds(value string) (int, error) {
	if duration, err := time.ParseDuration(value); err == nil {
		return int(duration.Seconds()), nil
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid interval %q: want duration (1s) or integer seconds", value)
	}
	return seconds, nil
}

// IntervalPtrSeconds конвертирует строку-длительность из файла конфигурации
// в указатель на секунды для Resolve; из nil-входа получается nil-результат.
func IntervalPtrSeconds(value *string) (*int, error) {
	if value == nil {
		return nil, nil
	}

	seconds, err := ParseIntervalSeconds(*value)
	if err != nil {
		return nil, err
	}
	return &seconds, nil
}
