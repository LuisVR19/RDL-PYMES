package organization

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func input() NewInput {
	return NewInput{LegalName: " Alfa S.A. ", IdentificationTypeCode: "02", IdentificationNumber: "3101123456", Email: "a@alfa.test"}
}

func TestNewNormalizesAndDefaultsTimezone(t *testing.T) {
	o, err := New(uuid.New(), input())
	if err != nil {
		t.Fatal(err)
	}
	if o.LegalName != "Alfa S.A." || o.Timezone != DefaultTimezone || o.Status != StatusActive {
		t.Fatalf("o=%+v", o)
	}
}

func TestNewReportsEveryInvalidField(t *testing.T) {
	_, err := New(uuid.New(), NewInput{IdentificationNumber: "31 01", Email: "x", Timezone: "Local"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err=%v", err)
	}
	fields := map[string]bool{}
	for _, e := range err.(interface{ Unwrap() []error }).Unwrap() {
		var fe FieldError
		if errors.As(e, &fe) {
			fields[fe.Field] = true
		}
	}
	for _, f := range []string{"legalName", "identificationTypeCode", "identificationNumber", "email", "timezone"} {
		if !fields[f] {
			t.Errorf("falta el error de %s (tiene %v)", f, fields)
		}
	}
}

func TestApplyPatch(t *testing.T) {
	o, _ := New(uuid.New(), input())
	name, empty, tz := "Alfa", "", "America/Panama"
	next, err := o.Apply(Patch{TradeName: &name, Phone: &empty, Timezone: &tz})
	if err != nil {
		t.Fatal(err)
	}
	if next.TradeName != "Alfa" || next.Timezone != "America/Panama" || next.LegalName != o.LegalName {
		t.Fatalf("next=%+v", next)
	}
	if o.TradeName != "" {
		t.Fatal("Apply no debe modificar el original")
	}
	bad := "Marte/Olympus"
	if _, err := o.Apply(Patch{Timezone: &bad}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err=%v", err)
	}
}
