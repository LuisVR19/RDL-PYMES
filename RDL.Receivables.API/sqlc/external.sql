-- Objetos que NO pertenecen a este repo (los crea database-platform o Platform). Solo sirven para que sqlc tipee
-- las consultas; nunca se aplican. Si cambian allá, actualizar aquí. De core solo las columnas que se leen.
create schema if not exists shared;
create schema if not exists core;
create schema if not exists receivables;
create schema if not exists audit;
create schema if not exists integration;

create domain shared.money_amount  as numeric(18, 5);
create domain shared.currency_code as char(3);

create function shared.current_organization_id() returns uuid language sql as $$ select null::uuid $$;
create function shared.current_user_id() returns uuid language sql as $$ select null::uuid $$;
create function shared.set_updated_at() returns trigger language plpgsql as $$ begin return new; end $$;

create table core.organizations (
  id       uuid primary key,
  timezone text not null,
  status   text not null
);

create table core.users (
  id               uuid primary key,
  external_subject text not null,
  status           text not null
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

create function core.find_user_by_subject(p_subject text) returns setof core.users
  language sql as $$ select * from core.users where external_subject = p_subject $$;

-- audit e integration: los crea database-platform (0006_audit_integration). Receivables solo inserta, y en el inbox
-- y la dead letter actualiza; las políticas filtran por shared.current_service().
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

create table integration.inbox_messages (
  consumer_service text not null,
  event_id         uuid not null,
  event_type       text not null,
  organization_id  uuid not null,
  received_at      timestamptz not null default now(),
  processed_at     timestamptz,
  primary key (consumer_service, event_id)
);

create table integration.dead_letters (
  id                  uuid primary key default gen_random_uuid(),
  consumer_service    text not null,
  event_id            uuid not null,
  event_type          text not null,
  organization_id     uuid,
  payload             jsonb not null,
  error_message       text not null,
  attempts            integer not null,
  failed_at           timestamptz not null default now(),
  resolved_at         timestamptz,
  resolved_by_user_id uuid,
  resolution_notes    text
);

create domain shared.exchange_rate as numeric(18, 5);

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

create table integration.idempotency_keys (
  service         text not null,
  organization_id uuid not null,
  idempotency_key text not null,
  request_hash    char(64) not null,
  response_status integer,
  response_body   jsonb,
  created_at      timestamptz not null default now(),
  expires_at      timestamptz not null,
  primary key (service, organization_id, idempotency_key)
);
