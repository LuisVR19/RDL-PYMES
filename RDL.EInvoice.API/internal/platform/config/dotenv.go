package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// LoadDotEnv carga un archivo .env si existe, solo para desarrollo local. Las variables ya definidas en el
// entorno tienen prioridad (en staging/producción vienen del orquestador y el archivo no existe).
// Formato: KEY=valor, KEY='valor literal', KEY="valor", comentarios con # al inicio de línea.
func LoadDotEnv(path string) error {
	f, err := os.Open(path) // #nosec G304 -- ruta fija del repo, solo desarrollo
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("abriendo %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	s := bufio.NewScanner(f)
	for n := 1; s.Scan(); n++ {
		// Quita el BOM que agrega el Bloc de notas de Windows al guardar en UTF-8.
		line := strings.TrimSpace(strings.TrimPrefix(s.Text(), "\xef\xbb\xbf"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		if !found || key == "" {
			// Sin repetir el contenido de la línea: podría ser una contraseña mal pegada.
			return fmt.Errorf("%s línea %d: se esperaba CLAVE=valor", path, n)
		}
		if _, set := os.LookupEnv(key); set {
			continue
		}
		v, err := unquote(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("%s línea %d (%s): %w", path, n, key, err)
		}
		if err := os.Setenv(key, v); err != nil {
			return err
		}
	}
	return s.Err()
}

func unquote(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	switch q := v[0]; q {
	case '\'', '"':
		end := strings.LastIndexByte(v, q)
		if end == 0 {
			return "", errors.New("comilla sin cerrar")
		}
		return v[1:end], nil
	default:
		// Sin comillas: un # precedido de espacio inicia un comentario.
		if i := strings.Index(v, " #"); i >= 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v), nil
	}
}
