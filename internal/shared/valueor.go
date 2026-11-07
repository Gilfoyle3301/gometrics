package shared

// ValueOr возвращает значение по указателю или defaultValue, если указатель nil.
// Применяется для приоритета конфигурации из окружения над значениями флагов.
func ValueOr[T any](ptr *T, defaultValue T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}
