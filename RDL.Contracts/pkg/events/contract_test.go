package events

import (
	"encoding/json"
	"io/fs"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	contracts "bitbucket.org/rdl/contracts"
	"bitbucket.org/rdl/contracts/pkg/events/money"
)

// registry: cada evento con su schema. Un evento nuevo se agrega aquí y recibe todos los tests de contrato.
var registry = map[string]reflect.Type{
	"schemas/events/envelope.v1.json":         reflect.TypeFor[Envelope](),
	InvoiceIssuedSpec.SchemaFile:              reflect.TypeFor[InvoiceIssuedV1](),
	InvoiceCancelledSpec.SchemaFile:           reflect.TypeFor[InvoiceCancelledV1](),
	CreditNoteIssuedSpec.SchemaFile:           reflect.TypeFor[CreditNoteIssuedV1](),
	DebitNoteIssuedSpec.SchemaFile:            reflect.TypeFor[DebitNoteIssuedV1](),
	ElectronicDocumentAcceptedSpec.SchemaFile: reflect.TypeFor[ElectronicDocumentAcceptedV1](),
	ElectronicDocumentRejectedSpec.SchemaFile: reflect.TypeFor[ElectronicDocumentRejectedV1](),
	PaymentReceivedSpec.SchemaFile:            reflect.TypeFor[PaymentReceivedV1](),
	ReceivableSettledSpec.SchemaFile:          reflect.TypeFor[ReceivableSettledV1](),
}

// leafByRef: qué tipo Go corresponde a cada tipo común del schema.
var leafByRef = map[string]reflect.Type{
	"money.json":         reflect.TypeFor[money.Amount](),
	"exchange-rate.json": reflect.TypeFor[money.ExchangeRate](),
	"quantity.json":      reflect.TypeFor[money.Quantity](),
	"percentage.json":    reflect.TypeFor[money.Percentage](),
	"currency-code.json": reflect.TypeFor[money.Currency](),
	"uuid.json":          reflect.TypeFor[uuid.UUID](),
	"utc-datetime.json":  reflect.TypeFor[Instant](),
	"business-date.json": reflect.TypeFor[Date](),
	"service-name.json":  reflect.TypeFor[Service](),
	"document-type.json": reflect.TypeFor[DocumentType](),
	"cabys-code.json":    reflect.TypeFor[string](),
	"fiscal-code.json":   reflect.TypeFor[string](),
}

type node = map[string]any

func loadSchema(t *testing.T, file string) node {
	t.Helper()
	raw, err := fs.ReadFile(contracts.Schemas, file)
	if err != nil {
		t.Fatal(err)
	}
	var n node
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatal(err)
	}
	return n
}

// objectShape junta properties y required de un schema, incluidos los de sus allOf.
func objectShape(t *testing.T, file string, n node) (props map[string]propRef, required []string) {
	props = map[string]propRef{}
	for _, part := range asSlice(n["allOf"]) {
		ref := part.(node)["$ref"].(string)
		f := path.Join(path.Dir(file), ref)
		p, r := objectShape(t, f, loadSchema(t, f))
		for k, v := range p {
			props[k] = v
		}
		required = append(required, r...)
	}
	for k, v := range asMap(n["properties"]) {
		if _, inherited := props[k]; inherited {
			// Un evento solo puede acotar un campo del sobre con const (eventType, version, sourceService).
			if !isConstOnly(v.(node)) {
				t.Errorf("%s: %s redefine un campo del sobre", file, k)
			}
			continue
		}
		props[k] = propRef{file: file, schema: v.(node)}
	}
	for _, r := range asSlice(n["required"]) {
		required = append(required, r.(string))
	}
	return props, required
}

type propRef struct {
	file   string
	schema node
}

func isConstOnly(n node) bool { _, ok := n["const"]; return ok && len(n) == 1 }

// jsonFields devuelve los campos JSON de un struct, aplanando los embebidos como hace encoding/json.
func jsonFields(t reflect.Type) map[string]reflect.StructField {
	out := map[string]reflect.StructField{}
	for f := range t.Fields() {
		if f.Anonymous {
			for k, v := range jsonFields(f.Type) {
				out[k] = v
			}
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out[name] = f
		}
	}
	return out
}

func omitempty(f reflect.StructField) bool { return strings.Contains(f.Tag.Get("json"), ",omitempty") }

func TestDTOsMatchSchemas(t *testing.T) {
	for file, typ := range registry {
		t.Run(path.Base(file), func(t *testing.T) { compareObject(t, file, loadSchema(t, file), typ, "") })
	}
}

func compareObject(t *testing.T, file string, n node, typ reflect.Type, at string) {
	t.Helper()
	props, required := objectShape(t, file, n)
	fields := jsonFields(typ)
	for name := range props {
		if _, ok := fields[name]; !ok {
			t.Errorf("%s%s: el schema tiene %q y %s no", file, at, name, typ.Name())
		}
	}
	for name, f := range fields {
		p, ok := props[name]
		if !ok {
			t.Errorf("%s%s: %s.%s (%q) no está en el schema", file, at, typ.Name(), f.Name, name)
			continue
		}
		req := slices.Contains(required, name)
		if req && omitempty(f) {
			t.Errorf("%s%s/%s: es required y el campo Go tiene omitempty", file, at, name)
		}
		if !req && !omitempty(f) && f.Type.Kind() != reflect.Pointer {
			t.Errorf("%s%s/%s: es opcional; el campo Go necesita omitempty", file, at, name)
		}
		compareValue(t, p.file, p.schema, f.Type, at+"/"+name)
	}
}

func compareValue(t *testing.T, file string, n node, typ reflect.Type, at string) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if ref, ok := n["$ref"].(string); ok {
		base := path.Base(ref)
		if want, leaf := leafByRef[base]; leaf {
			if typ != want {
				t.Errorf("%s%s: %s se mapea a %v, el campo es %v", file, at, base, want, typ)
			}
			return
		}
		f := path.Join(path.Dir(file), ref)
		compareValue(t, f, loadSchema(t, f), typ, at)
		return
	}
	switch {
	case n["type"] == "array":
		if typ.Kind() != reflect.Slice {
			t.Errorf("%s%s: el schema es un arreglo y el campo es %v", file, at, typ)
			return
		}
		compareValue(t, file, n["items"].(node), typ.Elem(), at+"/[]")
	case n["type"] == "object" || n["properties"] != nil || n["allOf"] != nil:
		compareObject(t, file, n, typ, at)
	case n["type"] == "string":
		if typ.Kind() != reflect.String {
			t.Errorf("%s%s: string en el schema, %v en Go", file, at, typ)
		}
	case n["type"] == "integer":
		if typ.Kind() != reflect.Int {
			t.Errorf("%s%s: integer en el schema, %v en Go", file, at, typ)
		}
	case n["type"] == "boolean":
		if typ.Kind() != reflect.Bool {
			t.Errorf("%s%s: boolean en el schema, %v en Go", file, at, typ)
		}
	}
}

// Ningún DTO puede tener float: el dinero y los decimales van en pkg/events/money.
func TestNoFloatsInDTOs(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type, string)
	walk = func(typ reflect.Type, at string) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		switch typ.Kind() {
		case reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
			t.Errorf("%s es %v: prohibido float", at, typ)
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(typ.Elem(), at)
		case reflect.Map:
			walk(typ.Key(), at)
			walk(typ.Elem(), at)
		case reflect.Struct:
			for f := range typ.Fields() {
				walk(f.Type, at+"."+f.Name)
			}
		}
	}
	for _, typ := range registry {
		walk(typ, typ.Name())
	}
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
