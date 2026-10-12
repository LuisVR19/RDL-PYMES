package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/adapters/http/problem"
	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/pkg/tenancy"
)

// FiscalStatusHandlers son las rutas internas del BFF (RDL.Contracts/openapi/bff-internal.yaml): el gateway reenvía
// el JWT del usuario y pregunta el estado fiscal de una factura o nota, o de varias a la vez.
type FiscalStatusHandlers struct {
	BySource interface {
		Execute(ctx context.Context, t tenancy.Context, sourceDocumentID uuid.UUID) (app.FiscalStatus, error)
	}
	BySources interface {
		Execute(ctx context.Context, t tenancy.Context, ids []uuid.UUID) ([]app.FiscalStatus, error)
	}
}

type fiscalStatusResponse struct {
	ElectronicDocumentID  uuid.UUID `json:"electronicDocumentId"`
	Status                string    `json:"status"`
	HaciendaStatusMessage string    `json:"haciendaStatusMessage,omitempty"`
}

type fiscalStatusItemResponse struct {
	SourceDocumentID uuid.UUID `json:"sourceDocumentId"`
	fiscalStatusResponse
}

type fiscalStatusBatchResponse struct {
	Items []fiscalStatusItemResponse `json:"items"`
}

type idBatchRequest struct {
	IDs []string `json:"ids"`
}

func (h *FiscalStatusHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /internal/v1/electronic-documents/by-source/{sourceDocumentId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "sourceDocumentId")
		if !ok {
			problem.Write(w, r, problem.NotFound)
			return
		}
		t, _ := tenancy.From(r.Context())
		s, err := h.BySource.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toFiscalStatusResponse(s))
	})
	rt.handle("POST /internal/v1/electronic-documents/by-source", func(w http.ResponseWriter, r *http.Request) {
		var in idBatchRequest
		if err := decodeJSON(w, r, &in); err != nil {
			fail(w, r, err)
			return
		}
		ids, err := parseIDBatch(in.IDs)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		statuses, err := h.BySources.Execute(r.Context(), t, ids)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := fiscalStatusBatchResponse{Items: make([]fiscalStatusItemResponse, 0, len(statuses))}
		for _, s := range statuses {
			out.Items = append(out.Items, fiscalStatusItemResponse{SourceDocumentID: s.SourceDocumentID, fiscalStatusResponse: toFiscalStatusResponse(s)})
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func toFiscalStatusResponse(s app.FiscalStatus) fiscalStatusResponse {
	return fiscalStatusResponse{ElectronicDocumentID: s.ElectronicDocumentID, Status: s.Status, HaciendaStatusMessage: s.HaciendaStatusMessage}
}

// parseIDBatch valida el IdBatch del contrato: de 1 a 100 UUID distintos. Todos los errores juntos en un 422.
func parseIDBatch(raw []string) ([]uuid.UUID, error) {
	var f fields
	switch {
	case len(raw) == 0:
		f.add("ids", "debe traer al menos un id")
	case len(raw) > app.MaxBatch:
		f.add("ids", fmt.Sprintf("admite hasta %d ids", app.MaxBatch))
	}
	ids := make([]uuid.UUID, 0, len(raw))
	seen := make(map[uuid.UUID]bool, len(raw))
	for i, v := range raw {
		field := fmt.Sprintf("ids[%d]", i)
		id := f.uuid(field, v)
		if id == uuid.Nil {
			continue
		}
		if seen[id] {
			f.add(field, "está repetido")
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, f.err()
}
