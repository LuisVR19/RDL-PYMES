package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/pkg/correlation"
	"rdl/billing-api/pkg/requestinfo"
)

type audit struct{ q *db.Queries }

// Record escribe el evento en la misma transacción que el cambio auditado.
func (a audit) Record(ctx context.Context, e app.AuditEvent) error {
	payload := map[string]any{}
	if e.Before != nil {
		payload["before"] = e.Before
	}
	if e.After != nil {
		payload["after"] = e.After
	}
	if e.Reason != "" {
		payload["reason"] = e.Reason
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serializando payload de auditoría: %w", err)
	}

	cid, ok := correlation.FromContext(ctx)
	if !ok {
		// Operaciones fuera de un request HTTP (workers futuros): igual deben quedar correlacionables.
		cid = uuid.New()
	}
	info := requestinfo.From(ctx)

	params := db.InsertAuditEventParams{
		OrganizationID: nullUUID(e.OrganizationID),
		ActorType:      "user",
		ActorUserID:    nullUUID(e.ActorUserID),
		Action:         e.Action,
		EntityType:     e.EntityType,
		EntityID:       nullUUID(e.EntityID),
		CorrelationID:  cid,
		UserAgent:      pgtype.Text{String: info.UserAgent, Valid: info.UserAgent != ""},
		Payload:        string(raw),
	}
	if e.ActorUserID == uuid.Nil {
		params.ActorType = "system"
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

func nullUUID(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: id != uuid.Nil}
}
