// Package examples lee los archivos de ejemplos de RDL.Contracts (examples/events/*.json): {"valid": [eventos],
// "invalid": [{"base": i, "patch": [...], "why": "..."}]}. Lo usan cmd/replay y los tests del consumidor, para
// probar con exactamente los mismos eventos que valida el repo de contratos.
package examples

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Invalid es un ejemplo inválido ya armado: el válido base con su patch aplicado.
type Invalid struct {
	Payload json.RawMessage
	Why     string
}

type file struct {
	Valid   []json.RawMessage `json:"valid"`
	Invalid []struct {
		Base  int       `json:"base"`
		Patch []patchOp `json:"patch"`
		Why   string    `json:"why"`
	} `json:"invalid"`
}

// IsExamplesFile indica si raw tiene la forma de un archivo de ejemplos (y no la de un evento suelto).
func IsExamplesFile(raw []byte) bool {
	var f struct {
		Valid json.RawMessage `json:"valid"`
	}
	return json.Unmarshal(raw, &f) == nil && f.Valid != nil
}

// Parse devuelve los ejemplos válidos tal cual y los inválidos con su patch aplicado.
func Parse(raw []byte) (valid []json.RawMessage, invalid []Invalid, err error) {
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, nil, fmt.Errorf("archivo de ejemplos: %w", err)
	}
	for i, inv := range f.Invalid {
		if inv.Base < 0 || inv.Base >= len(f.Valid) {
			return nil, nil, fmt.Errorf("invalid[%d]: base %d fuera de rango", i, inv.Base)
		}
		doc, err := decode(f.Valid[inv.Base])
		if err != nil {
			return nil, nil, fmt.Errorf("valid[%d]: %w", inv.Base, err)
		}
		patched, err := applyPatch(doc, inv.Patch)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid[%d]: %w", i, err)
		}
		out, err := json.Marshal(patched)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid[%d]: %w", i, err)
		}
		invalid = append(invalid, Invalid{Payload: out, Why: inv.Why})
	}
	return f.Valid, invalid, nil
}

// decode usa json.Number para que un número del ejemplo vuelva a serializarse igual (sin pasar por float64).
func decode(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	return v, dec.Decode(&v)
}
