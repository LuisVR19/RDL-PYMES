package events_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"bitbucket.org/rdl/contracts/pkg/events"
	"bitbucket.org/rdl/contracts/pkg/events/money"
)

// Así arma Billing el evento de emisión, la fila del outbox y la validación previa a escribirla.
func ExampleToOutboxRow() {
	issuedAt := events.NewInstant(time.Date(2026, 9, 24, 21, 15, 0, 0, time.UTC))
	today := events.DateIn(issuedAt.Time(), time.UTC)

	e := events.InvoiceIssuedV1{
		Envelope:  events.NewHeader(events.InvoiceIssuedSpec, uuid.New(), uuid.New(), issuedAt),
		InvoiceID: uuid.New(), InvoiceNumber: "FAC-0000001",
		IssuedAt: issuedAt, IssueDate: today, DueDate: today,
		Currency: money.MustCurrency("CRC"), ExchangeRate: money.MustExchangeRate("1"),
		// ... cliente, líneas y totales
	}
	row, err := events.ToOutboxRow(e)
	if err != nil {
		panic(err)
	}
	fmt.Println(row.EventType, row.EventVersion, row.AggregateType)

	v, err := events.DefaultValidator()
	if err != nil {
		panic(err)
	}
	// Este evento está incompleto (sin cliente ni líneas): el Validator lo rechaza antes de llegar al outbox.
	err = v.Validate(events.InvoiceIssuedSpec.SchemaFile, row.Payload)
	fmt.Println("rechazado:", errors.Is(err, events.ErrInvalid))
	// Output:
	// InvoiceIssued 1 invoice
	// rechazado: true
}
