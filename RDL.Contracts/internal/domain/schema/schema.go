// Package schema fija las reglas de identidad de los JSON Schema del repo: dialecto, $id y ubicación.
package schema

import (
	"path"
	"strings"
)

// Dialect es el único dialecto admitido (JSON Schema 2020-12).
const Dialect = "https://json-schema.org/draft/2020-12/schema"

// BaseURI es la raíz de los $id. El TLD .invalid (RFC 2606) garantiza que nunca se resuelva por red: los $ref se
// resuelven siempre contra los archivos del repo.
const BaseURI = "https://contracts.rdl.invalid/"

// Dir es la carpeta de los schemas dentro del repo.
const Dir = "schemas"

// ExpectedID es el $id que debe declarar el schema guardado en file (ruta relativa a la raíz del repo, con `/`).
// Que el $id refleje la ruta hace que un `$ref` relativo signifique lo mismo en el repo y en el validador.
func ExpectedID(file string) string {
	return BaseURI + path.Clean(file)
}

// FileForID es la inversa de ExpectedID; ok es false si el id no pertenece a este repo.
func FileForID(id string) (string, bool) {
	rest, ok := strings.CutPrefix(id, BaseURI)
	return rest, ok && rest != ""
}

// IsSchemaFile indica si la ruta es un schema del repo.
func IsSchemaFile(file string) bool {
	return strings.HasPrefix(file, Dir+"/") && strings.HasSuffix(file, ".json")
}
