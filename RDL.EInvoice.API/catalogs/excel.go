package catalogs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

// ExcelSource es un archivo de docs/hacienda/catalogos que se importa tal cual (sin transcribir): la ubicación de
// Hacienda y el CABYS del BCCR. El importador se niega a leerlo si su sha256 no es el esperado.
type ExcelSource struct {
	File    string // relativo a docs/hacienda/
	SHA256  string
	Version string
	Cite    string
}

var (
	// LocationSource: Anexo 1, Nota 14 (pág. 76) remite a «Codificacionubicacion_V4.4»; descargado el 2026-10-12 de
	// https://www.hacienda.go.cr/docs/Codificacionubicacion_V4.4.xlsx.
	LocationSource = ExcelSource{
		File:    "catalogos/Codificacionubicacion_V4.4.xlsx",
		SHA256:  "4fc2048e646b047baf76c41de8bf11f71b2385881d102b34fe72b3db6830e67d",
		Version: "Codificacionubicacion_V4.4",
		Cite:    "Anexo 1, Nota 14, pág. 76",
	}
	// CabysSource: Anexo 1, Nota 17 (pág. 80) remite al catálogo del BCCR. Versión 2025, publicada el 2025-04-01
	// (comunicado CP-BCCR-015-2025 en la misma carpeta); desde el 2025-06-01 solo se aceptan códigos de esta versión.
	CabysSource = ExcelSource{
		File:    "catalogos/CABYS_2025-04-01_catalogo-de-bienes-servicios.xlsx",
		SHA256:  "1b93554e37d274f086f2f63baa4dbda2c1d5f2e9798ddf8b3206d58baa789878",
		Version: "CABYS 2025 (BCCR, 2025-04-01)",
		Cite:    "Anexo 1, Nota 17, pág. 80; comunicado CP-BCCR-015-2025",
	}
)

// Open verifica el sha256 y abre el libro. dir es docs/hacienda/.
func (s ExcelSource) Open(dir string) (*excelize.File, error) {
	path := filepath.Join(dir, filepath.FromSlash(s.File))
	raw, err := os.ReadFile(path) // #nosec G304 -- archivo de docs/hacienda/ con hash verificado abajo
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != s.SHA256 {
		return nil, fmt.Errorf("%s cambió (sha256 %s, se esperaba %s): revise la fuente y actualice el manifiesto", s.File, got, s.SHA256)
	}
	return excelize.OpenFile(path)
}

// Location es una fila de provincia, cantón y distrito con sus códigos ya en el formato del XSD.
type Location struct {
	ProvinceCode, ProvinceName string
	CantonCode, CantonName     string
	DistrictCode, DistrictName string
}

var digitsRe = regexp.MustCompile(`^[0-9]+$`)

// pad deja el código con el ancho del XSD: Provincia \d, Canton y Distrito \d\d (FacturaElectronica_V4.4.xsd). El
// Excel los trae sin ceros a la izquierda.
func pad(code string, width int) (string, error) {
	code = strings.TrimSpace(code)
	if !digitsRe.MatchString(code) || len(code) > width {
		return "", fmt.Errorf("código %q inválido", code)
	}
	return strings.Repeat("0", width-len(code)) + code, nil
}

// LoadLocations lee la hoja de Codificacionubicacion_V4.4: columnas B a G (provincia, cantón y distrito con su
// nombre; H e I son barrios, que no se guardan). El mismo distrito se repite por cada barrio; de los nombres se
// prefiere el que no está todo en mayúsculas (la primera fila de cada provincia los trae en mayúsculas).
func LoadLocations(f *excelize.File) ([]Location, error) {
	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		return nil, err
	}
	byKey := map[string]*Location{}
	var order []string
	for i, r := range rows {
		if i == 0 || len(r) < 7 || strings.TrimSpace(r[1]) == "" {
			continue // encabezado o fila vacía
		}
		var l Location
		var errs []error
		var err error
		if l.ProvinceCode, err = pad(r[1], 1); err != nil {
			errs = append(errs, err)
		}
		if l.CantonCode, err = pad(r[3], 2); err != nil {
			errs = append(errs, err)
		}
		if l.DistrictCode, err = pad(r[5], 2); err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			return nil, fmt.Errorf("fila %d: %w", i+1, errors.Join(errs...))
		}
		l.ProvinceName, l.CantonName, l.DistrictName = strings.TrimSpace(r[2]), strings.TrimSpace(r[4]), strings.TrimSpace(r[6])
		key := l.ProvinceCode + "-" + l.CantonCode + "-" + l.DistrictCode
		prev, ok := byKey[key]
		if !ok {
			byKey[key] = &l
			order = append(order, key)
			continue
		}
		prev.ProvinceName = better(prev.ProvinceName, l.ProvinceName)
		prev.CantonName = better(prev.CantonName, l.CantonName)
		prev.DistrictName = better(prev.DistrictName, l.DistrictName)
	}
	out := make([]Location, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	if len(out) == 0 {
		return nil, errors.New("el archivo de ubicación no trae distritos")
	}
	return out, nil
}

func better(cur, cand string) string {
	if cur == "" || (cur == strings.ToUpper(cur) && cand != strings.ToUpper(cand)) {
		return cand
	}
	return cur
}

// CabysItem es un código de 13 dígitos con su descripción y la tarifa de IVA en puntos porcentuales. Cuando el BCCR no
// trae tarifa sino un texto («Exento», «na»), TaxRate es nil y TaxLabel guarda ese texto (migración 00003).
type CabysItem struct {
	Code        string
	Description string
	TaxRate     *decimal.Decimal
	TaxLabel    string
}

// cabysLabels son los únicos textos que el archivo trae en lugar de una tarifa. Otro texto es un error: así un
// cambio de formato del BCCR no pasa en silencio.
var cabysLabels = map[string]bool{"Exento": true, "na": true}

var cabysRe = regexp.MustCompile(`^[0-9]{13}$`)

// LoadCabys lee la hoja «Catálogo»: el código de 13 dígitos es la categoría 9 (columna Q), su descripción la R y el
// impuesto la S. Se guarda en puntos porcentuales (13 = 13 %).
func LoadCabys(f *excelize.File) ([]CabysItem, error) {
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	header := -1
	for i, r := range rows {
		if len(r) > 18 && strings.HasPrefix(r[16], "Categor") && strings.HasPrefix(r[18], "Impuesto") {
			header = i
			break
		}
	}
	if header < 0 {
		return nil, fmt.Errorf("hoja %q: no se encontró el encabezado (Categoría 9 … Impuesto)", sheet)
	}
	seen := map[string]bool{}
	var out []CabysItem
	for i, r := range rows[header+1:] {
		if len(r) < 19 || strings.TrimSpace(r[16]) == "" {
			continue
		}
		code := strings.TrimSpace(r[16])
		if !cabysRe.MatchString(code) {
			return nil, fmt.Errorf("fila %d: código CABYS %q inválido", header+i+2, code)
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		item := CabysItem{Code: code, Description: strings.TrimSpace(r[17])}
		if cell := strings.TrimSpace(r[18]); cabysLabels[cell] {
			item.TaxLabel = cell
		} else {
			rate, err := taxPoints(cell)
			if err != nil {
				return nil, fmt.Errorf("fila %d: impuesto %q inválido para %s", header+i+2, r[18], code)
			}
			item.TaxRate = &rate
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, errors.New("el CABYS no trae códigos")
	}
	return out, nil
}

// taxPoints lee el impuesto de la celda como texto: con el formato del libro («13%») ya viene en puntos; sin formato
// es la fracción guardada («0.13») y se lleva a puntos con Shift(2). Decimal exacto, nunca float.
func taxPoints(cell string) (decimal.Decimal, error) {
	cell = strings.TrimSpace(cell)
	var v decimal.Decimal
	var err error
	if num, ok := strings.CutSuffix(cell, "%"); ok {
		v, err = decimal.NewFromString(strings.TrimSpace(num))
	} else {
		v, err = decimal.NewFromString(cell)
		v = v.Shift(2)
	}
	if err != nil || v.IsNegative() || v.GreaterThan(decimal.NewFromInt(100)) {
		return decimal.Zero, fmt.Errorf("impuesto %q inválido", cell)
	}
	return v, nil
}
