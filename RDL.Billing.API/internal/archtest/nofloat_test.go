package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/product"
)

// Prohibido float en montos, cantidades, tasas o tipo de cambio (prompt P4, convenciones §3). La regla es más
// estricta: ningún float en el código de producción ni en el SQL. Si algún día hace falta uno que no sea dinero
// (una métrica, por ejemplo), se agrega aquí con su justificación.
var allowedFloatFiles = map[string]string{}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for !fileExists(filepath.Join(dir, "go.mod")) {
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no se encontró go.mod")
		}
		dir = parent
	}
	return dir
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestNoFloatInGoCode(t *testing.T) {
	root := repoRoot(t)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "bin") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if _, ok := allowedFloatFiles[rel]; ok {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && (id.Name == "float32" || id.Name == "float64") {
				t.Errorf("%s: usa %s; los montos, cantidades y tasas son decimales exactos", rel, id.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

var floatSQL = regexp.MustCompile(`(?i)\b(real|double\s+precision|float[48]?)\b`)

func TestNoFloatInSQL(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{"migrations", "queries"} {
		files, err := filepath.Glob(filepath.Join(root, dir, "*.sql"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			raw, err := os.ReadFile(file) // #nosec G304 -- archivos del repo
			if err != nil {
				t.Fatal(err)
			}
			for i, line := range strings.Split(string(raw), "\n") {
				code, _, _ := strings.Cut(line, "--")
				if floatSQL.MatchString(code) {
					t.Errorf("%s:%d: tipo flotante en SQL: %s", filepath.Base(file), i+1, strings.TrimSpace(line))
				}
			}
		}
	}
}

// TestNoFloatInDomainTypes recorre por reflexión los tipos del dominio (y lo que contienen) buscando floats.
// Los DTOs HTTP tienen el mismo test en su paquete (son privados).
func TestNoFloatInDomainTypes(t *testing.T) {
	for _, v := range []any{customer.Customer{}, product.Product{}, product.Patch{}, product.NewInput{},
		customer.NewInput{}, customer.Patch{}, money.Amount{}, money.Currency{}, money.Quantity{}, money.Percentage{},
		invoice.LineInput{}, invoice.LineAmounts{}, invoice.Totals{}} {
		if path, ok := FindFloat(reflect.TypeOf(v)); ok {
			t.Errorf("%T contiene un float en %s", v, path)
		}
	}
}

func TestFindFloatDetects(t *testing.T) {
	type inner struct{ Rate float64 }
	type outer struct{ Lines []*inner }
	if p, ok := FindFloat(reflect.TypeOf(outer{})); !ok || p != "outer.Lines[][].Rate" {
		t.Fatalf("FindFloat = %q, %v", p, ok)
	}
}
