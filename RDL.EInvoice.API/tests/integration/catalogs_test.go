//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// callList hace un GET que responde un arreglo (los catálogos no vienen envueltos en un objeto).
func callList(t *testing.T, path, token string) (int, []map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	raw, _ := io.ReadAll(rec.Body)
	var out []map[string]any
	_ = json.Unmarshal(raw, &out)
	return rec.Code, out
}

func byCode(items []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, it := range items {
		out[it["code"].(string)] = it
	}
	return out
}

func TestCabysSearch(t *testing.T) {
	token := env.A.OwnerToken(env)
	code, items := callList(t, "/v1/catalogs/cabys?q=labial", token)
	if code != http.StatusOK || byCode(items)["3532301000100"]["taxRate"] != "13" {
		t.Errorf("q=labial: %d %v", code, items)
	}
	code, items = callList(t, "/v1/catalogs/cabys?q=35323010001&limit=5", token)
	if code != http.StatusOK || len(items) == 0 || len(items) > 5 || items[0]["code"].(string)[:11] != "35323010001" {
		t.Errorf("por prefijo: %d %v", code, items)
	}
	// Un código exento trae su texto y no una tarifa inventada.
	code, items = callList(t, "/v1/catalogs/cabys?q=0211100010100", token)
	if code != http.StatusOK || len(items) != 1 || items[0]["taxLabel"] != "Exento" || items[0]["taxRate"] != nil {
		t.Errorf("exento: %d %v", code, items)
	}
	// % y _ no son comodines.
	if code, items = callList(t, "/v1/catalogs/cabys?q=%25%25", token); code != http.StatusOK || len(items) != 0 {
		t.Errorf("q=%%%%: %d, %d resultados", code, len(items))
	}
	for _, bad := range []string{"/v1/catalogs/cabys?q=a", "/v1/catalogs/cabys", "/v1/catalogs/cabys?q=trigo&limit=500"} {
		if r := env.Call(t, http.MethodGet, bad, token); r.Status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %v", bad, r.Status, r.Body)
		}
	}
}

func TestFiscalCatalogs(t *testing.T) {
	token := env.A.OwnerToken(env)
	code, rates := callList(t, "/v1/catalogs/tax-rates", token)
	if r := byCode(rates); code != http.StatusOK || len(rates) != 11 || r["08"]["rate"] != "13" || r["09"]["rate"] != "0.5" {
		t.Errorf("tax-rates: %d %v", code, rates)
	}
	code, conds := callList(t, "/v1/catalogs/sale-conditions", token)
	if c := byCode(conds); code != http.StatusOK || c["02"]["requiresCreditTerm"] != true || c["01"]["requiresCreditTerm"] != false {
		t.Errorf("sale-conditions: %d %v", code, conds)
	}
	code, units := callList(t, "/v1/catalogs/units-of-measure", token)
	if u := byCode(units); code != http.StatusOK || len(units) != 101 || u["cm"]["name"] != "centímetro" || u["Cm"]["name"] != "Comisiones" {
		t.Errorf("units-of-measure: %d, %d unidades", code, len(units))
	}
	code, currencies := callList(t, "/v1/catalogs/currencies", token)
	if code != http.StatusOK || len(currencies) != 168 || byCode(currencies)["CRC"]["name"] != "Colón costarricense" {
		t.Errorf("currencies: %d, %d monedas", code, len(currencies))
	}
	code, provinces := callList(t, "/v1/catalogs/provinces", token)
	if code != http.StatusOK || len(provinces) != 7 {
		t.Errorf("provinces: %d %v", code, provinces)
	}
	code, districts := callList(t, "/v1/catalogs/districts?provinceCode=1&cantonCode=01", token)
	if code != http.StatusOK || byCode(districts)["01"]["name"] != "Carmen" {
		t.Errorf("districts de San José: %d %v", code, districts)
	}
	if r := env.Call(t, http.MethodGet, "/v1/catalogs/cantons", token); r.Status != http.StatusUnprocessableEntity {
		t.Errorf("cantons sin provincia: %d %v", r.Status, r.Body)
	}
	if r := env.Call(t, http.MethodGet, "/v1/catalogs/no-existe", token); r.Status != http.StatusNotFound {
		t.Errorf("catálogo desconocido: %d %v", r.Status, r.Body)
	}
	// Los catálogos son públicos para cualquier miembro: B ve lo mismo.
	if code, b := callList(t, "/v1/catalogs/tax-rates", env.B.OwnerToken(env)); code != http.StatusOK || len(b) != 11 {
		t.Errorf("tax-rates desde B: %d %d", code, len(b))
	}
}
