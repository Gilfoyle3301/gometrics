package templates

import "embed"

// TemplFS — встроенный шаблон главной HTML-страницы сервера.
//
//go:embed dashboard.html
var TemplFS embed.FS
