package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"rdl/receivables-api/internal/adapters/postgres/db"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/pkg/requestinfo"
)

type audit struct{ q *db.Queries }

// Record escribe el evento en la misma transacción que el cambio auditado. El correlationId lo pone el caso de uso
// (el del request o el del evento de origen); IP y user-agent salen del request, si lo hay.
func (a audit) Record(ctx context.Context, e app.AuditEvent) error {
	payload := e.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serializando payload de auditoría: %w", err)
	}
	info := requestinfo.From(ctx)
	params := db.InsertAuditEventParams{
		OrganizationID: nullUUID(e.OrganizationID),
		ActorType:      e.ActorType,
		ActorUserID:    nullUUID(e.ActorUserID),
		Action:         e.Action,
		EntityType:     e.EntityType,
		EntityID:       nullUUID(e.EntityID),
		CorrelationID:  e.CorrelationID,
		UserAgent:      pgtype.Text{String: info.UserAgent, Valid: info.UserAgent != ""},
		Payload:        string(raw),
	}
	if info.IP.IsValid() {
		ip := info.IP
		params.IpAddress = &ip
	}
	if err := a.q.InsertAuditEvent(ctx, params); err != nil {
		return fmt.Errorf("registrando auditoría: %w", err)
	}
	return nil
}
