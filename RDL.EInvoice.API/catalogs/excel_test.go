package catalogs

import (
	"testing"

	"github.com/shopspring/decimal"
)

const haciendaDir = "../docs/hacienda"

func TestExcelSourcesAreInManifest(t *testing.T) {
	m := manifest(t)
	for _, s := range []ExcelSource{LocationSource, CabysSource} {
		if m[s.File] != s.SHA256 {
			t.Errorf("%s: sha256 %s, el manifiesto dice %q", s.File, s.SHA256, m[s.File])
		}
	}
}

func TestLoadLocations(t *testing.T) {
	f, err := LocationSource.Open(haciendaDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	locs, err := LoadLocations(f)
	if err != nil {
		t.Fatal(err)
	}
	provinces := map[string]string{}
	for _, l := range locs {
		provinces[l.ProvinceCode] = l.ProvinceName
		if len(l.ProvinceCode) != 1 || len(l.CantonCode) != 2 || len(l.DistrictCode) != 2 {
			t.Fatalf("códigos fuera del formato del XSD: %+v", l)
		}
	}
	// Costa Rica tiene 7 provincias; San José es la 1 y su primer distrito es Carmen.
	if len(provinces) != 7 || provinces["1"] != "San José" {
		t.Errorf("provincias = %v", provinces)
	}
	if first := locs[0]; first.CantonCode != "01" || first.DistrictCode != "01" || first.DistrictName != "Carmen" {
		t.Errorf("primer distrito = %+v", first)
	}
	if len(locs) < 450 || len(locs) > 520 {
		t.Errorf("%d distritos: se esperaban unos 490", len(locs))
	}
}

func TestLoadCabys(t *testing.T) {
	if testing.Short() {
		t.Skip("el CABYS tarda unos segundos en leerse")
	}
	f, err := CabysSource.Open(haciendaDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	items, err := LoadCabys(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 15000 {
		t.Errorf("%d códigos: el CABYS trae más de 20 000 filas", len(items))
	}
	byCode := map[string]CabysItem{}
	for _, it := range items {
		byCode[it.Code] = it
	}
	// Filas vistas en el archivo: trigo duro para siembra al 1 %, lápiz labial al 13 %.
	for code, rate := range map[string]string{"0111100000100": "1", "3532301000100": "13"} {
		it, ok := byCode[code]
		if !ok || it.TaxRate == nil || !it.TaxRate.Equal(decimal.RequireFromString(rate)) {
			t.Errorf("%s = %+v, se esperaba tarifa %s", code, it, rate)
		}
	}
	labels := map[string]int{}
	for _, it := range items {
		if it.TaxRate == nil {
			labels[it.TaxLabel]++
		}
	}
	if labels["Exento"] != 885 || labels["na"] != 3 || len(items) != 20506 {
		t.Errorf("%d códigos, textos = %v (se esperaban 20506, Exento 885, na 3)", len(items), labels)
	}
}

func TestTaxPoints(t *testing.T) {
	for in, want := range map[string]string{"13%": "13", "1%": "1", "0.5%": "0.5", "0.13": "13", "0.04": "4", "0": "0"} {
		got, err := taxPoints(in)
		if err != nil || !got.Equal(decimal.RequireFromString(want)) {
			t.Errorf("%q = %s, %v; se esperaba %s", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "trece", "-1%", "101%"} {
		if _, err := taxPoints(bad); err == nil {
			t.Errorf("%q aceptado", bad)
		}
	}
}
