package money

import (
	"errors"
	"fmt"
	"testing"

	cmoney "bitbucket.org/rdl/contracts/pkg/events/money"
	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

func TestCanonicalAmount(t *testing.T) {
	for in, want := range map[string]string{
		"0": "0", "0.00000": "0", "12": "12", "12.000": "12", "1300.50000": "1300.5", "0.00001": "0.00001",
		"100.10": "100.1", "9999999999999.99999": "9999999999999.99999",
	} {
		if got := CanonicalAmount(MustAmountForTest(in)).String(); got != want {
			t.Errorf("CanonicalAmount(%s) = %s, se esperaba %s", in, got, want)
		}
	}
}

func TestContractFormatsAreEnforced(t *testing.T) {
	for _, bad := range []string{"-1", "1e3", "1,000", "01", "1.123456", "12345678901234", " 1", ""} {
		if _, err := ParseAmount(bad); err == nil {
			t.Errorf("ParseAmount(%q) debió fallar", bad)
		}
	}
	for _, bad := range []string{"crc", "CR", "CRCC", ""} {
		if _, err := ParseCurrency(bad); err == nil {
			t.Errorf("ParseCurrency(%q) debió fallar", bad)
		}
	}
}

// Los casos de D2 (ADR 0007 del repo de contratos): se mira el sexto decimal; 5 o más sube.
func TestRoundHalfUpOnSixthDecimal(t *testing.T) {
	for in, want := range map[string]string{
		"0.1299987":            "0.13",
		"0.000005":             "0.00001", // mitad exacta: sube (el bancario la dejaría en 0)
		"0.000015":             "0.00002",
		"0.000025":             "0.00003", // el bancario daría 0.00002
		"0.0000049999":         "0",
		"1.123455":             "1.12346",
		"1.1234549":            "1.12345",
		"2.5":                  "2.5",
		"0":                    "0",
		"9999999999999.999994": "9999999999999.99999",
	} {
		if got := Round(decimal.RequireFromString(in)); !got.Equal(decimal.RequireFromString(want)) {
			t.Errorf("Round(%s) = %s, se esperaba %s", in, got, want)
		}
	}
}

func TestRoundRejectsNegative(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("un monto negativo debe hacer pánico: es un error de cálculo")
		}
	}()
	Round(decimal.RequireFromString("-0.1"))
}

func TestPercent(t *testing.T) {
	for _, c := range []struct{ base, rate, want string }{
		{"10000", "13", "1300"},
		{"0.99999", "13", "0.13"},     // 0.1299987
		{"1000", "6.5", "65"},         // exoneración: la mitad de un IVA de 13
		{"0.0005", "1", "0.00001"},    // 0.000005: mitad exacta
		{"3249.99", "4", "129.9996"},  // exacto, sin redondeo
		{"33.33333", "13", "4.33333"}, // 4.3333329
	} {
		got := Percent(decimal.RequireFromString(c.base), decimal.RequireFromString(c.rate))
		if !got.Equal(decimal.RequireFromString(c.want)) {
			t.Errorf("Percent(%s, %s) = %s, se esperaba %s", c.base, c.rate, got, c.want)
		}
	}
}

func TestToAmount(t *testing.T) {
	if a, err := ToAmount(decimal.RequireFromString("1300.50000")); err != nil || a.String() != "1300.5" {
		t.Fatalf("a=%v err=%v", a, err)
	}
	for _, bad := range []string{"-1", "10000000000000", "0.000001"} {
		if _, err := ToAmount(decimal.RequireFromString(bad)); !errors.Is(err, ErrInvalid) {
			t.Errorf("ToAmount(%s): err = %v", bad, err)
		}
	}
}

// El contrato (money.Round5, ADR 0007 de contratos) exige que cada API redondee igual que su función de referencia o
// con una equivalente probada contra los mismos casos: los del Anexo más cualquier decimal no negativo.
func TestRoundMatchesContractRound5(t *testing.T) {
	for _, s := range []string{"20.203512", "20.203518", "0.000005", "0.0000049999", "1.123455", "9999999999999.999994", "0", "12.5"} {
		assertSameRound(t, s)
	}
	rapid.Check(t, func(rt *rapid.T) {
		whole := rapid.Int64Range(0, 9_999_999_999_999).Draw(rt, "entero")
		frac := rapid.StringMatching(`[0-9]{0,9}`).Draw(rt, "decimales")
		s := fmt.Sprint(whole)
		if frac != "" {
			s += "." + frac
		}
		assertSameRound(rt, s)
	})
}

func assertSameRound(t interface{ Fatalf(string, ...any) }, s string) {
	want, err := cmoney.Round5(s)
	if err != nil {
		t.Fatalf("Round5(%s): %v", s, err)
	}
	if got := Round(decimal.RequireFromString(s)); !got.Equal(decimal.RequireFromString(want)) {
		t.Fatalf("Round(%s) = %s, el contrato da %s", s, got, want)
	}
}
