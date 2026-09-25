// Package aging clasifica saldos por antigüedad (decisión R6, ADR 0007). Funciones puras sobre fechas de negocio:
// asOf es un día del calendario en la zona de la organización, nunca un instante UTC.
package aging

import (
	"time"

	"rdl/receivables-api/internal/domain/civil"
)

// Bucket es un tramo. Los códigos son los de la vista receivables.receivable_aging.
type Bucket string

const (
	// Current: todavía no vence. Una cuenta que vence hoy no está atrasada.
	Current    Bucket = "current"
	Days1_30   Bucket = "1_30"
	Days31_60  Bucket = "31_60"
	Days61_90  Bucket = "61_90"
	Days90Plus Bucket = "90_plus"
)

// Buckets en el orden en que se muestran.
var Buckets = []Bucket{Current, Days1_30, Days31_60, Days61_90, Days90Plus}

// DaysOverdue son los días de atraso al cierre de asOf: 0 si todavía no venció.
func DaysOverdue(dueOn, asOf civil.Date) int {
	// Las dos fechas son medianoche UTC del mismo calendario: la diferencia es un número exacto de días.
	days := int(asOf.Time().Sub(dueOn.Time()) / (24 * time.Hour))
	return max(days, 0)
}

// For devuelve el tramo de una cuenta que vence dueOn, vista en asOf.
func For(dueOn, asOf civil.Date) Bucket {
	switch d := DaysOverdue(dueOn, asOf); {
	case d == 0:
		return Current
	case d <= 30:
		return Days1_30
	case d <= 60:
		return Days31_60
	case d <= 90:
		return Days61_90
	default:
		return Days90Plus
	}
}
