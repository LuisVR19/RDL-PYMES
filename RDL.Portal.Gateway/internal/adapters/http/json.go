package http

import (
	"encoding/json"
	"net/http"
)

// writeJSON serializa una respuesta propia del gateway (hoy, solo las composiciones).
// El paso directo no pasa por aquí: copia el cuerpo de la API sin tocarlo.
func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
