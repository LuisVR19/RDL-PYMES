// Package wiring arma los casos de uso con sus verificaciones. Agregar una regla es agregarla a una de estas listas.
package wiring

import (
	"io/fs"

	"bitbucket.org/rdl/contracts/internal/adapters/jsonschema"
	"bitbucket.org/rdl/contracts/internal/adapters/yamlfs"
	"bitbucket.org/rdl/contracts/internal/app"
)

// Validate: los artefactos están bien formados y son coherentes entre sí.
func Validate() *app.RunChecks {
	return app.NewRunChecks(
		app.NewOwnershipCheck(yamlfs.OwnershipLoader{}),
		app.NewSchemasCheck(jsonschema.Compiler{}),
		app.NewExamplesCheck(jsonschema.Compiler{}),
		app.NewCatalogCheck(yamlfs.AsyncAPILoader{}, jsonschema.AsyncAPI30()),
		app.NewStateMachinesCheck(yamlfs.StateMachineLoader{}),
		app.NewOpenAPICheck(yamlfs.DocumentLoader{}, jsonschema.OpenAPI31(), yamlfs.StateMachineLoader{}),
	)
}

// Breaking compara contra una versión base publicada.
func Breaking() *app.Breaking { return app.NewBreaking(yamlfs.DocumentLoader{}) }

// Diagram dibuja una máquina de estado en Mermaid.
func Diagram(repo fs.FS, file string) (string, error) {
	return app.Diagram(yamlfs.StateMachineLoader{}, repo, file)
}

// Lint: los artefactos cumplen las convenciones de docs/convenciones.md.
func Lint() *app.RunChecks {
	return app.NewRunChecks(
		app.NewLintCheck(yamlfs.DocumentLoader{}),
	)
}
