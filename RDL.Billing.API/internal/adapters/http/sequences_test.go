package http

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/numbering"
	"rdl/billing-api/pkg/tenancy"
)

type seqListFn func(context.Context, tenancy.Context) ([]numbering.Sequence, error)

func (fn seqListFn) Execute(ctx context.Context, t tenancy.Context) ([]numbering.Sequence, error) {
	return fn(ctx, t)
}

type seqConfigureFn func(context.Context, tenancy.Context, app.SequenceInput) (numbering.Sequence, error)

func (fn seqConfigureFn) Execute(ctx context.Context, t tenancy.Context, in app.SequenceInput) (numbering.Sequence, error) {
	return fn(ctx, t, in)
}

func newSequenceRouter(got *app.SequenceInput, called *bool, err error) http.Handler {
	return newRouterWith(Deps{Sequences: &SequenceHandlers{
		List: seqListFn(func(context.Context, tenancy.Context) ([]numbering.Sequence, error) {
			s := numbering.New(orgA, numbering.Scope{DocumentType: "invoice"})
			s.Prefix, s.NextNumber = "FAC-", 7
			last := int64(6)
			s.LastAssigned = &last
			return []numbering.Sequence{s}, err
		}),
		Configure: seqConfigureFn(func(_ context.Context, _ tenancy.Context, in app.SequenceInput) (numbering.Sequence, error) {
			*got, *called = in, true
			s := numbering.New(orgA, numbering.Scope{DocumentType: string(in.DocumentType), BranchID: in.BranchID})
			s.Prefix, s.NextNumber = in.Prefix, in.NextNumber
			return s, err
		}),
	}})
}

func TestConfigureSequenceHTTP(t *testing.T) {
	var got app.SequenceInput
	var called bool
	branch := uuid.New()
	rec := doBody(newSequenceRouter(&got, &called, nil), http.MethodPut, "/v1/document-sequences/credit_note", "tok-a",
		`{"branchId":"`+branch.String()+`","prefix":"NC-","nextNumber":100}`)
	if rec.Code != http.StatusOK || got.DocumentType != "credit_note" || *got.BranchID != branch || got.Prefix != "NC-" || got.NextNumber != 100 {
		t.Fatalf("status=%d input=%+v", rec.Code, got)
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if out["prefix"] != "NC-" || out["nextNumber"] != float64(100) || out["used"] != false {
		t.Fatalf("respuesta = %v", out)
	}
}

func TestConfigureSequenceErrorsHTTP(t *testing.T) {
	for name, tc := range map[string]struct {
		path, body  string
		err         error
		status      int
		problemType string
	}{
		"tipo desconocido":         {"/v1/document-sequences/receipt", `{"prefix":"A","nextNumber":1}`, nil, http.StatusNotFound, "not-found"},
		"tipo distinto a la ruta":  {"/v1/document-sequences/invoice", `{"documentType":"debit_note","prefix":"A","nextNumber":1}`, nil, http.StatusUnprocessableEntity, "validation"},
		"sin prefijo":              {"/v1/document-sequences/invoice", `{"nextNumber":1}`, nil, http.StatusUnprocessableEntity, "validation"},
		"número como string":       {"/v1/document-sequences/invoice", `{"prefix":"A","nextNumber":"1"}`, nil, http.StatusBadRequest, "malformed-request"},
		"en uso":                   {"/v1/document-sequences/invoice", `{"prefix":"A","nextNumber":1}`, numbering.ErrInUse, http.StatusConflict, "sequence-in-use"},
		"prefijo de otra sucursal": {"/v1/document-sequences/invoice", `{"prefix":"A","nextNumber":1}`, numbering.ErrPrefixTaken, http.StatusConflict, "conflict"},
		"rol sin permiso":          {"/v1/document-sequences/invoice", `{"prefix":"A","nextNumber":1}`, app.ErrForbidden, http.StatusForbidden, "forbidden"},
		"dominio":                  {"/v1/document-sequences/invoice", `{"prefix":"A","nextNumber":1}`, numbering.FieldError{Field: "prefix", Message: "x"}, http.StatusUnprocessableEntity, "validation"},
	} {
		t.Run(name, func(t *testing.T) {
			var got app.SequenceInput
			var called bool
			rec := doBody(newSequenceRouter(&got, &called, tc.err), http.MethodPut, tc.path, "tok-a", tc.body)
			if p := problemOf(t, rec); rec.Code != tc.status || p.Type != problem.TypeBase+tc.problemType {
				t.Fatalf("status=%d type=%s", rec.Code, p.Type)
			}
		})
	}
}

func TestListSequencesHTTP(t *testing.T) {
	var got app.SequenceInput
	var called bool
	rec := do(newSequenceRouter(&got, &called, nil), http.MethodGet, "/v1/document-sequences", "tok-a")
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if rec.Code != http.StatusOK || len(out) != 1 || out[0]["used"] != true || out[0]["nextNumber"] != float64(7) {
		t.Fatalf("status=%d %v", rec.Code, out)
	}
}
