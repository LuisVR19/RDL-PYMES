// Package migrations expone las migraciones goose de core y subscriptions embebidas en el binario.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
