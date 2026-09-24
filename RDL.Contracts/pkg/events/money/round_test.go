package money

import (
	"errors"
	"testing"
)

func TestRoundHalfUpAnexoExamples(t *testing.T) {
	cases := []struct {
		in     string
		places int
		want   string
	}{
		{"20.203512", 5, "20.20351"}, // ejemplo del Anexo 1: el sexto decimal es menor que 5
		{"20.203518", 5, "20.20352"}, // ejemplo del Anexo 1: el sexto decimal es 5 o más
		{"20.203515", 5, "20.20352"}, // exactamente 5: sube (no es redondeo bancario)
		{"20.203525", 5, "20.20353"}, // también sube con dígito par delante
		{"9.999995", 5, "10"},        // el acarreo cruza el entero
		{"0.000004", 5, "0"},
		{"0.000005", 5, "0.00001"},
		{"129.9987", 5, "129.9987"}, // ya tiene menos decimales: no cambia
		{"1300.50", 5, "1300.5"},    // normaliza ceros a la derecha
		{"11300", 5, "11300"},
		{"2.345", 2, "2.35"},
		{"2.344", 2, "2.34"},
		{"0.5", 0, "1"},
	}
	for _, c := range cases {
		got, err := RoundHalfUp(c.in, c.places)
		if err != nil || got != c.want {
			t.Errorf("RoundHalfUp(%q, %d) = %q, %v; se esperaba %q", c.in, c.places, got, err, c.want)
		}
		if c.places == Places {
			if _, err := ParseAmount(got); err != nil {
				t.Errorf("%q no cumple el patrón de Money: %v", got, err)
			}
		}
	}
}

func TestRoundHalfUpRejectsInvalid(t *testing.T) {
	for _, in := range []string{"", "-1.5", "1e3", "1,5", ".5", "abc"} {
		if _, err := Round5(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("Round5(%q) debería rechazarse: %v", in, err)
		}
	}
}
