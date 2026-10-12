-- Expand: el CABYS 2025 del BCCR (docs/hacienda/catalogos) trae en la columna Impuesto una tarifa ("0.13", "13%") o
-- un texto: "Exento" (885 códigos) o "na" (3). Exento no es lo mismo que 0 %: corresponde a la tarifa 10 «Tarifa
-- Exenta» de la Nota 8.1 (Anexo 1, pág. 72), distinta de la 01 «Tarifa 0 %». Se guarda el texto tal como viene y la
-- tarifa queda NULL; nunca se inventa una tarifa. Columna nueva y opcional: no cambia nada de lo existente.

-- +goose Up
alter table fiscal.cabys_items add column if not exists tax_label text;
-- +goose StatementBegin
do $$
begin
  if not exists (select 1 from pg_constraint where conname = 'cabys_items_tax_ck') then
    alter table fiscal.cabys_items
      add constraint cabys_items_tax_ck check (tax_rate is not null or tax_label is not null) not valid;
    alter table fiscal.cabys_items validate constraint cabys_items_tax_ck;
  end if;
end $$;
-- +goose StatementEnd
comment on column fiscal.cabys_items.tax_rate is 'Tarifa de IVA en puntos porcentuales (13 = 13 %), si el BCCR trae una.';
comment on column fiscal.cabys_items.tax_label is 'Texto del BCCR cuando no hay tarifa ("Exento", "na"). Exento ≈ tarifa 10 de la Nota 8.1.';

-- +goose Down
alter table fiscal.cabys_items drop constraint if exists cabys_items_tax_ck;
alter table fiscal.cabys_items drop column if exists tax_label;
