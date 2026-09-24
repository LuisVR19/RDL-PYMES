package money

import (
	"fmt"
	"math/big"
	"strings"
)

// Places es la escala de los montos del Anexo 1 v4.4 (Decimal 18,5) y de shared.money_amount.
const Places = 5

// RoundHalfUp redondea un decimal no negativo (string, sin exponente) a `places` decimales con el método del
// Anexo 1 v4.4 de Hacienda: se mira el decimal siguiente; si es menor que 5 no cambia, si es 5 o más sube una unidad.
// Ejemplos del anexo: 20.203512 → 20.20351 y 20.203518 → 20.20352. FUENTE: borrador sept-2024.
//
// Es la única aritmética de este paquete: existe para que todas las APIs redondeen igual. Cada API sigue calculando
// con su propia librería decimal y usa esta función (o una equivalente probada contra los mismos casos) para el
// resultado final de cada campo.
func RoundHalfUp(s string, places int) (string, error) {
	if places < 0 {
		return "", fmt.Errorf("%w: places negativo", ErrInvalid)
	}
	intPart, frac, _ := strings.Cut(s, ".")
	if intPart == "" || strings.Trim(intPart, "0123456789") != "" || strings.Trim(frac, "0123456789") != "" {
		return "", fmt.Errorf("%w: %q no es un decimal no negativo", ErrInvalid, s)
	}
	if len(frac) <= places {
		return normalize(intPart, frac), nil
	}
	keep, next := frac[:places], frac[places]
	n, _ := new(big.Int).SetString(intPart+keep, 10)
	if next >= '5' {
		n.Add(n, big.NewInt(1))
	}
	digits := n.String()
	if len(digits) <= places {
		digits = strings.Repeat("0", places-len(digits)+1) + digits
	}
	cut := len(digits) - places
	return normalize(digits[:cut], digits[cut:]), nil
}

// Round5 redondea a la escala de los montos (5 decimales).
func Round5(s string) (string, error) { return RoundHalfUp(s, Places) }

// normalize quita ceros sobrantes a la izquierda del entero y a la derecha de los decimales, para que el resultado
// cumpla los patrones de los schemas ("0", "1300.5", nunca "01300" ni "1300.").
func normalize(intPart, frac string) string {
	intPart = strings.TrimLeft(intPart, "0")
	if intPart == "" {
		intPart = "0"
	}
	frac = strings.TrimRight(frac, "0")
	if frac == "" {
		return intPart
	}
	return intPart + "." + frac
}
