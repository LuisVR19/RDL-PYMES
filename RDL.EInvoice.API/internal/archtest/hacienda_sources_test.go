package archtest

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docs/hacienda es la fuente de verdad fiscal (prompt P5): cada PDF y XSD está inventariado en manifest.sha256 y no
// cambia sin que cambie su hash. Un archivo nuevo sin registrar o uno modificado hace fallar el build.
func TestHaciendaSourcesMatchManifest(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "docs", "hacienda")
	f, err := os.Open(filepath.Join(dir, "manifest.sha256")) // #nosec G304 -- archivo del repo
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	want := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		sum, name, ok := strings.Cut(sc.Text(), "  ")
		if !ok {
			t.Fatalf("línea inválida en manifest.sha256: %q", sc.Text())
		}
		want[name] = sum
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".pdf" && ext != ".xsd" && ext != ".xlsx" && ext != ".xls" && ext != ".csv" {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		raw, err := os.ReadFile(path) // #nosec G304 G122 -- archivos del repo, sin symlinks
		if err != nil {
			return err
		}
		got := sha256.Sum256(raw)
		switch sum, ok := want[rel]; {
		case !ok:
			t.Errorf("%s no está en docs/hacienda/manifest.sha256", rel)
		case sum != hex.EncodeToString(got[:]):
			t.Errorf("%s cambió: su sha256 no coincide con el manifiesto", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("%s está en el manifiesto pero no en docs/hacienda", name)
		}
	}
}
