package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/adapters/postgres/db"
)

type organizations struct{ q *db.Queries }

func (r organizations) Timezone(ctx context.Context, org uuid.UUID) (string, error) {
	tz, err := r.q.GetOrganizationTimezone(ctx, org)
	if err != nil {
		// Sin fila no es un 404: la organización de la sesión ya se revalidó en el middleware.
		return "", fmt.Errorf("leyendo zona horaria de la organización: %w", err)
	}
	return tz, nil
}
