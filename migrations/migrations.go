// Package migrations содержит встроенные SQL-миграции схемы базы данных (goose).
package migrations

import "embed"

// FS — встроенные SQL-миграции схемы базы данных (goose).
//
//go:embed *.sql
var FS embed.FS
