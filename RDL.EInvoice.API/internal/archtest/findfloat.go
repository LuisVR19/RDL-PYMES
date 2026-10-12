// Package archtest verifica reglas de arquitectura que ningún linter cubre. FindFloat se exporta para que los
// paquetes con tipos privados (DTOs HTTP) apliquen la misma regla en sus propios tests.
package archtest

import "reflect"

// FindFloat devuelve la ruta del primer campo float dentro de t, siguiendo punteros, slices, mapas y structs.
func FindFloat(t reflect.Type) (string, bool) {
	return findFloat(t, t.Name(), map[reflect.Type]bool{})
}

func findFloat(t reflect.Type, path string, seen map[reflect.Type]bool) (string, bool) {
	if seen[t] {
		return "", false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Float32, reflect.Float64:
		return path, true
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return findFloat(t.Elem(), path+"[]", seen)
	case reflect.Map:
		if p, ok := findFloat(t.Key(), path+"{key}", seen); ok {
			return p, true
		}
		return findFloat(t.Elem(), path+"{}", seen)
	case reflect.Struct:
		for i := range t.NumField() {
			f := t.Field(i)
			if p, ok := findFloat(f.Type, path+"."+f.Name, seen); ok {
				return p, true
			}
		}
	}
	return "", false
}
