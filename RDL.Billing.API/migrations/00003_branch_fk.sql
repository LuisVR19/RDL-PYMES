-- FK compuestas de branch_id hacia core.branches (informe 0001 §3.2). Requiere, de la propuesta 0002, USAGE en el
-- schema core y REFERENCES en core.branches para billing_migrator. Expand: solo agrega restricciones; NOT VALID + VALIDATE para no bloquear la tabla
-- mientras se revisan las filas existentes. Con branch_id NULL la FK no se evalúa (MATCH SIMPLE).

-- +goose Up
-- +goose StatementBegin
do $$
begin
  if not exists (select 1 from pg_constraint where conname = 'invoices_branch_fk') then
    alter table billing.invoices
      add constraint invoices_branch_fk foreign key (organization_id, branch_id)
      references core.branches (organization_id, id) not valid;
  end if;
  if not exists (select 1 from pg_constraint where conname = 'document_sequences_branch_fk') then
    alter table billing.document_sequences
      add constraint document_sequences_branch_fk foreign key (organization_id, branch_id)
      references core.branches (organization_id, id) not valid;
  end if;
end $$;
-- +goose StatementEnd
alter table billing.invoices validate constraint invoices_branch_fk;
alter table billing.document_sequences validate constraint document_sequences_branch_fk;
create index if not exists invoices_branch_idx on billing.invoices (organization_id, branch_id);

-- +goose Down
alter table billing.document_sequences drop constraint if exists document_sequences_branch_fk;
alter table billing.invoices drop constraint if exists invoices_branch_fk;
drop index if exists billing.invoices_branch_idx;
