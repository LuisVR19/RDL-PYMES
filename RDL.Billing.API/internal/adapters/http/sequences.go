package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/numbering"
	"rdl/billing-api/pkg/tenancy"
)

type (
	listSequences interface {
		Execute(ctx context.Context, t tenancy.Context) ([]numbering.Sequence, error)
	}
	configureSequence interface {
		Execute(ctx context.Context, t tenancy.Context, in app.SequenceInput) (numbering.Sequence, error)
	}
)

type SequenceHandlers struct {
	List      listSequences
	Configure configureSequence
}

// DocumentSequence del contrato, más used y updatedAt.
type sequenceDTO struct {
	DocumentType string     `json:"documentType"`
	BranchID     *uuid.UUID `json:"branchId,omitempty"`
	Prefix       string     `json:"prefix"`
	NextNumber   int64      `json:"nextNumber"`
	Used         bool       `json:"used"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type configureSequenceRequest struct {
	DocumentType *string    `json:"documentType"`
	BranchID     *uuid.UUID `json:"branchId"`
	Prefix       *string    `json:"prefix" validate:"required"`
	NextNumber   *int64     `json:"nextNumber" validate:"required"`
}

func (h *SequenceHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/document-sequences", func(w http.ResponseWriter, r *http.Request) {
		t, _ := tenancy.From(r.Context())
		seqs, err := h.List.Execute(r.Context(), t)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := make([]sequenceDTO, 0, len(seqs))
		for _, s := range seqs {
			out = append(out, toSequenceDTO(s))
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("PUT /v1/document-sequences/{documentType}", func(w http.ResponseWriter, r *http.Request) {
		docType := invoice.DocumentType(r.PathValue("documentType"))
		switch docType {
		case invoice.TypeInvoice, invoice.TypeCreditNote, invoice.TypeDebitNote:
		default:
			problem.Write(w, r, problem.NotFound)
			return
		}
		var req configureSequenceRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		if req.DocumentType != nil && invoice.DocumentType(*req.DocumentType) != docType {
			fail(w, r, validationError{fields: []problem.FieldError{{Field: "documentType", Message: "debe coincidir con el de la ruta"}}})
			return
		}
		t, _ := tenancy.From(r.Context())
		s, err := h.Configure.Execute(r.Context(), t, app.SequenceInput{
			DocumentType: docType, BranchID: req.BranchID, Prefix: *req.Prefix, NextNumber: *req.NextNumber,
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toSequenceDTO(s))
	})
}

func toSequenceDTO(s numbering.Sequence) sequenceDTO {
	return sequenceDTO{
		DocumentType: s.DocumentType, BranchID: s.BranchID, Prefix: s.Prefix, NextNumber: s.NextNumber,
		Used: s.Used(), UpdatedAt: s.UpdatedAt.UTC(),
	}
}
