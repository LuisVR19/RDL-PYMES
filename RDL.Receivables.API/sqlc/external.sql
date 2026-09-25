-- Objetos que NO pertenecen a este repo (los crea database-platform o Platform). Solo sirven para que sqlc tipee
-- las consultas; nunca se aplican. Si cambian allá, actualizar aquí. De core solo las columnas que se leen.
create schema if not exists shared;
create schema if not exists core;
create schema if not exists receivables;

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
