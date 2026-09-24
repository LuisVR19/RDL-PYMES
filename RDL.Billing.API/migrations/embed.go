// Package migrations expone las migraciones goose de billing embebidas en el binario.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
