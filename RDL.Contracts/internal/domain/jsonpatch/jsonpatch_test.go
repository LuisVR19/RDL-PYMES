package jsonpatch

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func parse(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestApply(t *testing.T) {
	doc := parse(t, `{"a":{"b":1},"l":[{"x":1},{"x":2}],"k~/":0}`)
	cases := []struct {
		name string
		ops  []Op
		want string
	}{
		{"replace anidado", []Op{{Op: "replace", Path: "/a/b", Value: "1"}}, `{"a":{"b":"1"},"l":[{"x":1},{"x":2}],"k~/":0}`},
		{"remove en objeto", []Op{{Op: "remove", Path: "/a"}}, `{"l":[{"x":1},{"x":2}],"k~/":0}`},
		{"add nuevo campo", []Op{{Op: "add", Path: "/a/c", Value: true}}, `{"a":{"b":1,"c":true},"l":[{"x":1},{"x":2}],"k~/":0}`},
		{"remove en arreglo", []Op{{Op: "remove", Path: "/l/0"}}, `{"a":{"b":1},"l":[{"x":2}],"k~/":0}`},
		{"append", []Op{{Op: "add", Path: "/l/-", Value: 3.0}}, `{"a":{"b":1},"l":[{"x":1},{"x":2},3],"k~/":0}`},
		{"dentro de arreglo", []Op{{Op: "replace", Path: "/l/1/x", Value: 9.0}}, `{"a":{"b":1},"l":[{"x":1},{"x":9}],"k~/":0}`},
		{"escape de pointer", []Op{{Op: "replace", Path: "/k~0~1", Value: 1.0}}, `{"a":{"b":1},"l":[{"x":1},{"x":2}],"k~/":1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Apply(doc, c.ops)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, parse(t, c.want)) {
				b, _ := json.Marshal(got)
				t.Fatalf("got %s", b)
			}
		})
	}
	if !reflect.DeepEqual(doc, parse(t, `{"a":{"b":1},"l":[{"x":1},{"x":2}],"k~/":0}`)) {
		t.Fatal("Apply modificó el documento original")
	}
}

func TestApplyErrors(t *testing.T) {
	doc := parse(t, `{"a":{"b":1},"l":[1]}`)
	for name, ops := range map[string][]Op{
		"replace inexistente": {{Op: "replace", Path: "/z", Value: 1}},
		"remove inexistente":  {{Op: "remove", Path: "/a/z"}},
		"índice fuera":        {{Op: "replace", Path: "/l/5", Value: 1}},
		"op desconocida":      {{Op: "move", Path: "/a"}},
		"path sin /":          {{Op: "remove", Path: "a"}},
		"raíz":                {{Op: "replace", Path: "", Value: 1}},
		"padre inexistente":   {{Op: "add", Path: "/x/y", Value: 1}},
	} {
		if _, err := Apply(doc, ops); !errors.Is(err, ErrInvalidPatch) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
