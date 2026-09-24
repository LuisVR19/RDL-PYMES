// Command contractsctl valida los contratos del repo y detecta cambios incompatibles.
//
//	contractsctl validate [-root .] [-format text|json]
//	contractsctl lint     [-root .] [-format text|json]
//	contractsctl breaking -base <dir> [-root .] [-format text|json]
//	contractsctl diagram [-root .] state-machines/<entidad>.yaml
//	contractsctl version
//
// Códigos de salida: 0 sin errores, 1 hay hallazgos de error, 2 uso incorrecto o fallo de infraestructura.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"

	"bitbucket.org/rdl/contracts/internal/adapters/report"
	"bitbucket.org/rdl/contracts/internal/app"
	"bitbucket.org/rdl/contracts/internal/platform/buildinfo"
	"bitbucket.org/rdl/contracts/internal/wiring"
)

const (
	exitOK       = 0
	exitFindings = 1
	exitUsage    = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "version":
		_, _ = fmt.Fprintln(stdout, buildinfo.Current())
		return exitOK
	case "validate":
		return runChecks(ctx, cmd, wiring.Validate(), rest, stdout, stderr)
	case "lint":
		return runChecks(ctx, cmd, wiring.Lint(), rest, stdout, stderr)
	case "diagram":
		return runDiagram(rest, stdout, stderr)
	case "breaking":
		return runBreaking(ctx, rest, stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return exitOK
	}
	_, _ = fmt.Fprintf(stderr, "comando desconocido %q\n", cmd)
	usage(stderr)
	return exitUsage
}

type checksRunner interface {
	Execute(ctx context.Context, repo fs.FS) (app.Result, error)
}

func runChecks(ctx context.Context, name string, uc checksRunner, args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet(name, flag.ContinueOnError)
	fl.SetOutput(stderr)
	root := fl.String("root", ".", "raíz del repo de contratos")
	format := fl.String("format", string(report.Text), "salida: text o json")
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	f, err := report.ParseFormat(*format)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitUsage
	}

	res, err := uc.Execute(ctx, os.DirFS(*root))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return exitUsage
	}
	if err := report.Write(stdout, f, name, res); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: escribiendo el reporte: %v\n", name, err)
		return exitUsage
	}
	if res.Failed() {
		return exitFindings
	}
	return exitOK
}

// runBreaking compara el repo con una versión base publicada: un directorio con el mismo layout (por ejemplo el
// último tag descomprimido). El CI decide cómo obtenerlo.
func runBreaking(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("breaking", flag.ContinueOnError)
	fl.SetOutput(stderr)
	root := fl.String("root", ".", "raíz del repo de contratos (versión nueva)")
	base := fl.String("base", "", "directorio con la versión publicada contra la que se compara (obligatorio)")
	format := fl.String("format", string(report.Text), "salida: text o json")
	if err := fl.Parse(args); err != nil || *base == "" {
		_, _ = fmt.Fprintln(stderr, "uso: contractsctl breaking -base <directorio de la versión publicada> [-root .] [-format text|json]")
		return exitUsage
	}
	f, err := report.ParseFormat(*format)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitUsage
	}
	res, err := wiring.Breaking().Execute(ctx, os.DirFS(*base), os.DirFS(*root))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "breaking: %v\n", err)
		return exitUsage
	}
	if err := report.Write(stdout, f, "breaking", res); err != nil {
		_, _ = fmt.Fprintf(stderr, "breaking: escribiendo el reporte: %v\n", err)
		return exitUsage
	}
	if res.Failed() {
		return exitFindings
	}
	return exitOK
}

// runDiagram imprime el bloque Mermaid de una máquina de estado, para pegarlo en docs/maquinas-de-estado/.
func runDiagram(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("diagram", flag.ContinueOnError)
	fl.SetOutput(stderr)
	root := fl.String("root", ".", "raíz del repo de contratos")
	if err := fl.Parse(args); err != nil || fl.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "uso: contractsctl diagram [-root .] state-machines/<entidad>.yaml")
		return exitUsage
	}
	out, err := wiring.Diagram(os.DirFS(*root), fl.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "diagram: %v\n", err)
		return exitUsage
	}
	_, _ = fmt.Fprint(stdout, out)
	return exitOK
}

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `uso: contractsctl <comando> [flags]

comandos:
  validate   artefactos bien formados y coherentes (schemas, ejemplos, AsyncAPI, OpenAPI, máquinas de estado)
  lint       convenciones (dinero como string, fechas UTC, camelCase, sobre común...)
  diagram    bloque Mermaid de una máquina de estado (diagram state-machines/<entidad>.yaml)
  breaking   cambios incompatibles contra una versión publicada (-base <directorio>)
  version    versión del CLI
`)
}
