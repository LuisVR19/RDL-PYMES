-- Objetos que NO pertenecen a este repo (los crea database-platform). Solo sirven para que sqlc tipee las
-- consultas; nunca se aplican. Si database-platform los cambia, actualizar aquí.
create schema if not exists shared;
create schema if not exists core;
create schema if not exists subscriptions;
create schema if not exists audit;
create schema if not exists integration;

create domain shared.money_amount  as numeric(18, 5);
create domain shared.currency_code as char(3);
create domain shared.sha256_hex    as char(64);

create function shared.current_organization_id() returns uuid language sql as $$ select null::uuid $$;
create function shared.current_user_id() returns uuid language sql as $$ select null::uuid $$;
create function shared.current_service() returns text language sql as $$ select null::text $$;
create function shared.set_updated_at() returns trigger language plpgsql as $$ begin return new; end $$;

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
