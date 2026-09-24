-- Objetos que NO pertenecen a este repo (los crean database-platform y Platform). Solo sirven para que sqlc tipee
-- las consultas; nunca se aplican. Si cambian en la base, actualizar aquí. Solo las columnas que Billing puede leer.
create schema if not exists shared;
create schema if not exists core;
create schema if not exists billing;
create schema if not exists audit;
create schema if not exists integration;

create domain shared.money_amount  as numeric(18, 5);
create domain shared.quantity      as numeric(16, 3);
create domain shared.percentage    as numeric(7, 4);
create domain shared.exchange_rate as numeric(18, 5);
create domain shared.currency_code as char(3);
create domain shared.cabys_code    as char(13);
create domain shared.sha256_hex    as char(64);

create function shared.current_organization_id() returns uuid language sql as $$ select null::uuid $$;
create function shared.current_user_id() returns uuid language sql as $$ select null::uuid $$;
create function shared.current_service() returns text language sql as $$ select null::text $$;
create function shared.set_updated_at() returns trigger language plpgsql as $$ begin return new; end $$;

-- core: lectura para revalidar membresía y validar sucursales (docs/decisiones/0002).
create table core.users (
  id     uuid primary key,
  status text not null
);

create function core.find_user_by_subject(p_subject text) returns setof core.users
  language sql as $$ select * from core.users where false $$;

create table core.organizations (
  id                    uuid primary key,
  status                text not null,
  timezone              text not null,
  default_currency_code shared.currency_code not null
);

create table core.organization_users (
  id              uuid primary key,
  organization_id uuid not null,
  user_id         uuid not null,
  status          text not null
);

create table core.organization_user_roles (
  organization_id      uuid not null,
  organization_user_id uuid not null,
  role_code            text not null
);

create table core.branches (
  id              uuid primary key,
  organization_id uuid not null,
  code            text not null,
  name            text not null,
  is_active       boolean not null
);

-- fiscal: catálogo de tarifas de impuesto (solo lectura; lo mantiene fiscal).
create schema if not exists fiscal;
create table fiscal.tax_rates (
  code      text primary key,
  name      text not null,
  rate      shared.percentage not null,
  is_active boolean not null
);

create table audit.audit_events (
  id              uuid primary key default gen_random_uuid(),
  organization_id uuid,
  occurred_at     timestamptz not null default now(),
  service         text not null,
  actor_type      text not null,
  actor_user_id   uuid,
  action          text not null,
  entity_type     text not null,
  entity_id       uuid,
  correlation_id  uuid not null,
  ip_address      inet,
  user_agent      text,
  payload         jsonb not null default '{}'::jsonb
);

create table integration.idempotency_keys (
  service         text not null,
  organization_id uuid not null,
  idempotency_key text not null,
  request_hash    shared.sha256_hex not null,
  response_status integer,
  response_body   jsonb,
  created_at      timestamptz not null default now(),
  expires_at      timestamptz not null,
  primary key (service, organization_id, idempotency_key)
);

create table integration.outbox_messages (
  id               uuid primary key default gen_random_uuid(),
  source_service   text not null,
  organization_id  uuid not null,
  event_type       text not null,
  event_version    integer not null,
  aggregate_type   text not null,
  aggregate_id     uuid not null,
  correlation_id   uuid not null,
  payload          jsonb not null,
  occurred_at      timestamptz not null default now(),
  published_at     timestamptz,
  publish_attempts integer not null default 0,
  next_attempt_at  timestamptz,
  last_error       text
);

-- goose crea su tabla de historial antes de la baseline (que le quita privilegios a billing_app).
create table billing.goose_db_version (
  id         serial primary key,
  version_id bigint not null,
  is_applied boolean not null,
  tstamp     timestamp default now()
);
