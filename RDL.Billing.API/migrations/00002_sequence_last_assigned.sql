-- Marca de uso de una secuencia de numeración (informe 0001 §4.5, ADR 0006). PUT /v1/document-sequences solo
-- configura una secuencia que nunca asignó un número: con next_number solo no se distingue "configurada para arrancar
-- en 1000" de "ya emitió 999 documentos". Expand: columna nueva que admite NULL (= nunca usada); ningún dato cambia.

-- +goose Up
alter table billing.document_sequences add column if not exists last_assigned_number bigint;
-- +goose StatementBegin
do $$
begin
  if not exists (select 1 from pg_constraint where conname = 'document_sequences_last_assigned_ck') then
    alter table billing.document_sequences add constraint document_sequences_last_assigned_ck
      check (last_assigned_number is null or (last_assigned_number > 0 and last_assigned_number < next_number));
  end if;
end $$;
-- +goose StatementEnd
comment on column billing.document_sequences.last_assigned_number is
  'Último número asignado al emitir; NULL = la secuencia nunca se usó y todavía se puede configurar.';

-- +goose Down
alter table billing.document_sequences drop constraint if exists document_sequences_last_assigned_ck;
alter table billing.document_sequences drop column if exists last_assigned_number;
