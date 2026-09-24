package invoice

import (
	"fmt"
	"math/big"
	"testing"

	"pgregory.net/rapid"

	"rdl/billing-api/internal/domain/money"
)

// Tests basados en propiedades (definición de terminado del planning, P4). Para cualquier conjunto de líneas válidas:
//  1. la suma de las líneas es igual a los totales;
//  2. subtotal − descuento + impuesto = total, en cada línea y en la factura;
//  3. ningún redondeo pierde ni inventa céntimos (contra una referencia racional exacta);
//  4. el resultado no depende del orden de las líneas;
//  5. ningún monto es negativo.

// halfUnit es la mayor diferencia que puede introducir un redondeo a 5 decimales.
var halfUnit = big.NewRat(5, 1_000_000)

// fixed genera un decimal con `places` decimales entre 0 y max (en unidades de 10^-places).
func fixed(t *rapid.T, label string, maxUnits int64, places int) string {
	u := rapid.Int64Range(0, maxUnits).Draw(t, label)
	return new(big.Rat).SetFrac(big.NewInt(u), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)).FloatString(places)
}

func genTax(t *rapid.T, typeCode string) TaxInput {
	// Tarifas de IVA de Costa Rica más cualquier tarifa de 0 a 100 con 4 decimales (shared.percentage).
	rate := rapid.OneOf(
		rapid.SampledFrom([]string{"0", "1", "2", "4", "8", "13"}),
		rapid.Custom(func(t *rapid.T) string { return fixed(t, "rate", 1_000_000, 4) }),
	).Draw(t, "rate")
	tax := TaxInput{TypeCode: typeCode, Rate: money.MustPercentageForTest(trim(rate))}
	if rapid.Bool().Draw(t, "exonerated") {
		// Tarifa exonerada 4,2 y nunca mayor que la tarifa del impuesto: hasta floor(rate × 100) centésimas, tope 99.99.
		maxCents := min(money.Dec(tax.Rate).Shift(2).IntPart(), 9999)
		ex := fixed(t, "exoneratedRate", maxCents, 2)
		p := money.MustTaxRateForTest(trim(ex))
		tax.ExoneratedRate = &p
	}
	return tax
}

func genLine(t *rapid.T) LineInput {
	qty := fixed(t, "quantity", 99_999_999, 3) // hasta 99 999.999
	if rat(stringer(qty)).Sign() == 0 {
		qty = "0.001" // shared.quantity es > 0
	}
	price := fixed(t, "unitPrice", 999_999_999_999, 5) // hasta 9 999 999.99999: el bruto cabe en 13 enteros
	in := LineInput{
		Quantity:  money.MustQuantityForTest(trim(qty)),
		UnitPrice: money.MustAmountForTest(trim(price)),
		Discount:  money.MustAmountForTest("0"),
	}
	// Descuento entre 0 y el bruto redondeado.
	gross := money.Round(money.Dec(in.Quantity).Mul(money.Dec(in.UnitPrice)))
	grossUnits := gross.Shift(5).IntPart()
	if rapid.Bool().Draw(t, "hasDiscount") && grossUnits > 0 {
		in.Discount = money.MustAmountForTest(trim(fixed(t, "discount", grossUnits, 5)))
	}
	for _, code := range rapid.SliceOfNDistinct(rapid.SampledFrom([]string{"01", "02", "03", "04"}), 0, 3, rapid.ID).Draw(t, "taxTypes") {
		in.Taxes = append(in.Taxes, genTax(t, code))
	}
	return in
}

type stringer string

func (s stringer) String() string { return string(s) }

func trim(s string) string { return money.CanonicalAmount(money.MustAmountForTest(s)).String() }

func calcAll(t *rapid.T, in []LineInput) ([]LineAmounts, Totals) {
	lines := make([]LineAmounts, 0, len(in))
	for _, l := range in {
		a, err := CalculateLine(l)
		if err != nil {
			t.Fatalf("línea válida rechazada: %v (%+v)", err, l)
		}
		lines = append(lines, a)
	}
	tot, err := CalculateTotals(lines)
	if err != nil {
		t.Fatalf("totales: %v", err)
	}
	return lines, tot
}

func TestPropertyCalculation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		in := rapid.SliceOfN(rapid.Custom(genLine), 1, 8).Draw(t, "lines")
		lines, tot := calcAll(t, in)

		sub, disc, tax, exo, total := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
		for i, l := range lines {
			// 5. nada negativo (Amount no lo admite, pero se verifica igual contra la referencia).
			for _, v := range []money.Amount{l.Gross, l.Discount, l.Subtotal, l.Tax, l.Total} {
				if rat(v).Sign() < 0 {
					t.Fatalf("línea %d: monto negativo %s", i, v)
				}
			}
			// 2. por línea: bruto − descuento + impuesto neto = total.
			lhs := new(big.Rat).Add(new(big.Rat).Sub(rat(l.Gross), rat(l.Discount)), rat(l.Tax))
			if lhs.Cmp(rat(l.Total)) != 0 || new(big.Rat).Sub(rat(l.Gross), rat(l.Discount)).Cmp(rat(l.Subtotal)) != 0 {
				t.Fatalf("línea %d no cuadra: %+v", i, l)
			}

			// 3. contra la referencia exacta: cada redondeo se aleja a lo sumo media unidad del quinto decimal.
			exactGross := new(big.Rat).Mul(rat(in[i].Quantity), rat(in[i].UnitPrice))
			within(t, fmt.Sprintf("línea %d bruto", i), rat(l.Gross), exactGross, 1)
			lineTax, lineExo := new(big.Rat), new(big.Rat)
			for j, tx := range in[i].Taxes {
				exactTax := new(big.Rat).Quo(new(big.Rat).Mul(rat(l.Subtotal), rat(tx.Rate)), big.NewRat(100, 1))
				within(t, fmt.Sprintf("línea %d impuesto %d", i, j), rat(l.Taxes[j].Amount), exactTax, 1)
				if tx.ExoneratedRate != nil {
					exactExo := new(big.Rat).Quo(new(big.Rat).Mul(rat(l.Subtotal), rat(*tx.ExoneratedRate)), big.NewRat(100, 1))
					within(t, fmt.Sprintf("línea %d exoneración %d", i, j), rat(l.Taxes[j].Exoneration), exactExo, 1)
					if rat(l.Taxes[j].Exoneration).Cmp(rat(l.Taxes[j].Amount)) > 0 {
						t.Fatalf("línea %d: exoneración mayor que el impuesto", i)
					}
				}
				lineTax.Add(lineTax, rat(l.Taxes[j].Amount))
				lineExo.Add(lineExo, rat(l.Taxes[j].Exoneration))
			}
			if new(big.Rat).Sub(lineTax, lineExo).Cmp(rat(l.Tax)) != 0 {
				t.Fatalf("línea %d: impuesto neto ≠ Σ impuestos − Σ exoneraciones", i)
			}
			// El total de línea es exacto salvo los redondeos que lo componen: bruto + cada impuesto y exoneración.
			exactTotal := new(big.Rat).Sub(exactGross, rat(l.Discount))
			for _, tx := range in[i].Taxes {
				base := new(big.Rat).Sub(exactGross, rat(l.Discount))
				exactTotal.Add(exactTotal, new(big.Rat).Quo(new(big.Rat).Mul(base, rat(tx.Rate)), big.NewRat(100, 1)))
				if tx.ExoneratedRate != nil {
					exactTotal.Sub(exactTotal, new(big.Rat).Quo(new(big.Rat).Mul(base, rat(*tx.ExoneratedRate)), big.NewRat(100, 1)))
				}
			}
			// Cota: media unidad por redondeo propio, más el bruto propagado por las tarifas (≤ 100 % cada una).
			within(t, fmt.Sprintf("línea %d total", i), rat(l.Total), exactTotal, int64(1+4*len(in[i].Taxes)))

			sub.Add(sub, rat(l.Subtotal))
			disc.Add(disc, rat(l.Discount))
			tax.Add(tax, lineTax)
			exo.Add(exo, lineExo)
			total.Add(total, rat(l.Total))
		}

		// 1. Σ líneas = totales, exacto (las sumas no redondean).
		for name, pair := range map[string][2]*big.Rat{
			"subtotal": {rat(tot.Subtotal), sub}, "discount": {rat(tot.Discount), disc}, "tax": {rat(tot.Tax), tax},
			"exoneration": {rat(tot.Exoneration), exo}, "total": {rat(tot.Total), total},
		} {
			if pair[0].Cmp(pair[1]) != 0 {
				t.Fatalf("%s: total %s ≠ Σ líneas %s", name, pair[0].FloatString(5), pair[1].FloatString(5))
			}
		}
		// 2. en la factura: subtotal + impuesto − exoneración = total.
		if new(big.Rat).Sub(new(big.Rat).Add(sub, tax), exo).Cmp(rat(tot.Total)) != 0 {
			t.Fatal("subtotal + impuesto − exoneración ≠ total")
		}

		// 4. el orden de las líneas no cambia los totales (ni la línea calculada de cada una).
		perm := rapid.Permutation(in).Draw(t, "permutación")
		_, tot2 := calcAll(t, perm)
		if tot2 != tot {
			t.Fatalf("totales dependen del orden: %+v ≠ %+v", tot, tot2)
		}
	})
}

// TestPropertyDeterministic: el cálculo es una función pura.
func TestPropertyDeterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		in := genLine(t)
		a, errA := CalculateLine(in)
		b, errB := CalculateLine(in)
		if (errA == nil) != (errB == nil) || fmt.Sprint(a) != fmt.Sprint(b) {
			t.Fatalf("mismo input, distinto resultado: %+v / %+v", a, b)
		}
	})
}

// within falla si |got − exact| > n medias unidades del quinto decimal.
func within(t *rapid.T, what string, got, exact *big.Rat, n int64) {
	diff := new(big.Rat).Sub(got, exact)
	diff.Abs(diff)
	if diff.Cmp(new(big.Rat).Mul(halfUnit, big.NewRat(n, 1))) > 0 {
		t.Fatalf("%s: %s se aleja %s del valor exacto %s (máximo %d × 0.000005)",
			what, got.FloatString(5), diff.FloatString(8), exact.FloatString(8), n)
	}
}
