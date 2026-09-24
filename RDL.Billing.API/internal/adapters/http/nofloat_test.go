package http

import (
	"reflect"
	"testing"

	"rdl/billing-api/internal/archtest"
)

func TestNoFloatInDTOs(t *testing.T) {
	for _, v := range []any{customerResponse{}, customerPageResponse{}, createCustomerRequest{}, updateCustomerRequest{},
		productResponse{}, productPageResponse{}, createProductRequest{}, updateProductRequest{},
		invoiceResponse{}, invoicePageResponse{}, createInvoiceRequest{}, updateInvoiceRequest{}, invoiceLineRequest{},
		statusChangeDTO{}, sequenceDTO{}, configureSequenceRequest{}} {
		if path, ok := archtest.FindFloat(reflect.TypeOf(v)); ok {
			t.Errorf("%T contiene un float en %s", v, path)
		}
	}
}
