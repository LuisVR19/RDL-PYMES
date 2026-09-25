// Package migrations expone las migraciones goose del schema receivables embebidas en el binario.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

// BaselineVersion es la migración que reproduce el schema creado por database-platform. En dev se registra como
// aplicada con `migrate mark-baseline`, sin ejecutarla.
const BaselineVersion int64 = 1
