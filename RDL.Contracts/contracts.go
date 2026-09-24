// Package contracts expone los artefactos del repo embebidos en el binario, para que las APIs validen contra la misma
// versión de los schemas que importan (pkg/events.Validator los usa).
package contracts

import "embed"

// Schemas contiene schemas/**: JSON Schema 2020-12 de eventos y tipos comunes.
//
//go:embed schemas
var Schemas embed.FS
