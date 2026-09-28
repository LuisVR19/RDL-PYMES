-- Expand: índices que faltan para paginar por cursor y para el detalle de un pago
-- (docs/decisiones/0001-estado-inicial-bd.md §4.7). Solo agrega; no cambia datos ni restricciones.
-- Sin `concurrently`: goose corre cada migración en una transacción y las tablas están vacías en dev.

-- +goose Up
create index if not exists payment_applications_payment_idx
  on receivables.payment_applications (organization_id, payment_id);
create index if not exists receivables_created_idx
  on receivables.receivables (organization_id, created_at desc, id desc);
create index if not exists receivables_status_created_idx
  on receivables.receivables (organization_id, status, created_at desc, id desc);
create index if not exists payments_created_idx
  on receivables.payments (organization_id, created_at desc, id desc);

-- +goose Down
drop index if exists receivables.payments_created_idx;
drop index if exists receivables.receivables_status_created_idx;
drop index if exists receivables.receivables_created_idx;
drop index if exists receivables.payment_applications_payment_idx;
