-- Organización activa elegida por el usuario (docs/decisiones/0001-estado-inicial-bd.md §6.2).
-- Guardarla no concede acceso: el hook de Auth y el middleware revalidan la membresía activa.

-- +goose Up
alter table core.users
  add column if not exists active_organization_id uuid references core.organizations (id);
create index if not exists users_active_organization_idx on core.users (active_organization_id);
comment on column core.users.active_organization_id is
  'Organización elegida por el usuario. El hook de Auth la revalida contra una membresía activa antes de emitir org_id.';

-- +goose Down
drop index if exists core.users_active_organization_idx;
alter table core.users drop column if exists active_organization_id;
