package shared

import "fmt"

// FormatBuildInfo возвращает build-информацию приложения в формате трёх строк:
// Build version, Build date, Build commit. Пустые значения заменяются на "N/A".
func FormatBuildInfo(version, date, commit string) string {
	return fmt.Sprintf("Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		orNA(version), orNA(date), orNA(commit))
}

func orNA(value string) string {
	if value == "" {
		return "N/A"
	}
	return value
}
