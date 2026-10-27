// Package templates содержит встроенные HTML-шаблоны страниц сервера.
package templates

import "embed"

// TemplFS — встроенный шаблон главной HTML-страницы сервера.
//
//go:embed dashboard.html
var TemplFS embed.FS
