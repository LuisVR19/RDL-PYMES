package finding

import (
	"slices"
	"testing"
)

func TestSortIsDeterministic(t *testing.T) {
	in := []Finding{
		Errorf("r2", "b.json", "/x", "m"),
		Errorf("r1", "a.json", "/y", "m"),
		Errorf("r1", "a.json", "/x", "m2"),
		Errorf("r1", "a.json", "/x", "m1"),
	}
	Sort(in)
	got := make([]string, len(in))
	for i, f := range in {
		got[i] = f.File + f.Pointer + f.Message
	}
	want := []string{"a.json/xm1", "a.json/xm2", "a.json/ym", "b.json/xm"}
	if !slices.Equal(got, want) {
		t.Fatalf("orden = %v, se esperaba %v", got, want)
	}
}

func TestHasErrorsIgnoresWarnings(t *testing.T) {
	if HasErrors([]Finding{Warnf("todo", "a", "", "pendiente")}) {
		t.Fatal("un warning no debe contar como error")
	}
	if !HasErrors([]Finding{Warnf("todo", "a", "", ""), Errorf("r", "a", "", "")}) {
		t.Fatal("se esperaba error")
	}
}
