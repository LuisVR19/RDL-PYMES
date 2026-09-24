-- Baseline de core y subscriptions.
-- Reproduce database-platform 0002_core_subscriptions + la parte de subscriptions de 0008_catalog_policies.
-- Es idempotente: sobre la base dev existente no cambia nada; sobre una base vacía (tests) crea el esquema.
-- Requiere que el bootstrap de database-platform (roles, schemas, shared.*) ya exista.
-- Ver docs/decisiones/0001-estado-inicial-bd.md §6.1.

-- +goose Up
create table if not exists core.roles (
  code        text primary key,
  name        text not null,
  description text,
  created_at  timestamptz not null default now(),
  constraint roles_code_format_ck check (code ~ '^[a-z][a-z_]*$')
);
comment on table core.roles is 'Catálogo global de roles asignables dentro de una organización.';

create table if not exists core.organizations (
  id                       uuid primary key default gen_random_uuid(),
  legal_name               text not null,
  trade_name               text,
  identification_type_code text not null,
  identification_number    text not null,
  email                    text not null,
  phone                    text,
  timezone                 text not null default 'America/Costa_Rica',
  default_currency_code    shared.currency_code not null default 'CRC',
  status                   text not null default 'active',
  created_at               timestamptz not null default now(),
  updated_at               timestamptz not null default now(),
  constraint organizations_status_ck check (status in ('active', 'suspended', 'closed')),
  constraint organizations_identification_uk unique (identification_type_code, identification_number)
);
comment on table core.organizations is 'Tenant del SaaS. Toda tabla de negocio referencia su id en organization_id.';

create table if not exists core.users (
  id                uuid primary key default gen_random_uuid(),
  identity_provider text not null,
  external_subject  text not null,
  email             text not null,
  full_name         text not null,
  status            text not null default 'active',
  last_login_at     timestamptz,
  created_at        timestamptz not null default now(),
  updated_at        timestamptz not null default now(),
  constraint users_status_ck check (status in ('active', 'disabled')),
  constraint users_external_subject_uk unique (identity_provider, external_subject)
);
create unique index if not exists users_email_uk on core.users (lower(email));
comment on table core.users is 'Usuario global enlazado al proveedor de identidad OIDC (sub). Sin contraseñas.';

create table if not exists core.organization_users (
  id              uuid primary key default gen_random_uuid(),
  organization_id uuid not null references core.organizations (id),
  user_id         uuid not null references core.users (id),
  status          text not null default 'active',
  joined_at       timestamptz not null default now(),
  created_at      timestamptz not null default now(),
  updated_at      timestamptz not null default now(),
  constraint organization_users_status_ck check (status in ('active', 'suspended')),
  constraint organization_users_org_id_uk unique (organization_id, id),
  constraint organization_users_org_user_uk unique (organization_id, user_id)
);
create index if not exists organization_users_user_idx on core.organization_users (user_id);

create table if not exists core.organization_user_roles (
  organization_id      uuid not null,
  organization_user_id uuid not null,
  role_code            text not null references core.roles (code),
  granted_by_user_id   uuid references core.users (id),
  granted_at           timestamptz not null default now(),
  constraint organization_user_roles_pk primary key (organization_id, organization_user_id, role_code),
  constraint organization_user_roles_membership_fk foreign key (organization_id, organization_user_id)
    references core.organization_users (organization_id, id) on delete cascade
);
create index if not exists organization_user_roles_role_idx       on core.organization_user_roles (role_code);
create index if not exists organization_user_roles_granted_by_idx on core.organization_user_roles (granted_by_user_id);

create table if not exists core.invitations (
  id                  uuid primary key default gen_random_uuid(),
  organization_id     uuid not null references core.organizations (id),
  email               text not null,
  role_code           text not null references core.roles (code),
  token_hash          shared.sha256_hex not null,
  status              text not null default 'pending',
  expires_at          timestamptz not null,
  invited_by_user_id  uuid not null references core.users (id),
  accepted_by_user_id uuid references core.users (id),
  accepted_at         timestamptz,
  created_at          timestamptz not null default now(),
  updated_at          timestamptz not null default now(),
  constraint invitations_status_ck check (status in ('pending', 'accepted', 'revoked', 'expired')),
  constraint invitations_accepted_ck check ((status = 'accepted') = (accepted_at is not null and accepted_by_user_id is not null)),
  constraint invitations_org_id_uk unique (organization_id, id),
  constraint invitations_token_uk unique (token_hash)
);
create unique index if not exists invitations_pending_email_uk on core.invitations (organization_id, lower(email)) where status = 'pending';
create index if not exists invitations_role_idx        on core.invitations (role_code);
create index if not exists invitations_invited_by_idx  on core.invitations (invited_by_user_id);
create index if not exists invitations_accepted_by_idx on core.invitations (accepted_by_user_id);

create table if not exists core.branches (
  id              uuid primary key default gen_random_uuid(),
  organization_id uuid not null references core.organizations (id),
  code            text not null,
  name            text not null,
  address         text,
  phone           text,
  email           text,
  is_active       boolean not null default true,
  created_at      timestamptz not null default now(),
  updated_at      timestamptz not null default now(),
  constraint branches_org_id_uk unique (organization_id, id),
  constraint branches_org_code_uk unique (organization_id, code)
);

create or replace trigger organizations_set_updated_at      before update on core.organizations      for each row execute function shared.set_updated_at();
create or replace trigger users_set_updated_at              before update on core.users              for each row execute function shared.set_updated_at();
create or replace trigger organization_users_set_updated_at before update on core.organization_users for each row execute function shared.set_updated_at();
create or replace trigger invitations_set_updated_at        before update on core.invitations        for each row execute function shared.set_updated_at();
create or replace trigger branches_set_updated_at           before update on core.branches           for each row execute function shared.set_updated_at();

alter table core.roles                   enable row level security;
alter table core.organizations           enable row level security;
alter table core.users                   enable row level security;
alter table core.organization_users      enable row level security;
alter table core.organization_user_roles enable row level security;
alter table core.invitations             enable row level security;
alter table core.branches                enable row level security;

insert into core.roles (code, name, description) values
  ('owner',      'Propietario',   'Control total de la organización, incluida la suscripción.'),
  ('admin',      'Administrador', 'Gestiona usuarios, sucursales y configuración.'),
  ('biller',     'Facturador',    'Crea y emite facturas y notas.'),
  ('collector',  'Cobrador',      'Registra pagos y gestiona la cobranza.'),
  ('accountant', 'Contador',      'Consulta y exporta información contable y fiscal.'),
  ('read_only',  'Solo lectura',  'Consulta sin permisos de modificación.')
on conflict (code) do nothing;

create table if not exists subscriptions.plans (
  id                     uuid primary key default gen_random_uuid(),
  code                   text not null,
  name                   text not null,
  description            text,
  monthly_document_limit integer,
  price_amount           shared.money_amount not null,
  currency_code          shared.currency_code not null default 'CRC',
  is_active              boolean not null default true,
  created_at             timestamptz not null default now(),
  updated_at             timestamptz not null default now(),
  constraint plans_code_uk unique (code),
  constraint plans_document_limit_ck check (monthly_document_limit is null or monthly_document_limit > 0)
);
comment on column subscriptions.plans.monthly_document_limit is 'NULL = ilimitado.';

create table if not exists subscriptions.organization_subscriptions (
  id                   uuid primary key default gen_random_uuid(),
  organization_id      uuid not null references core.organizations (id),
  plan_id              uuid not null references subscriptions.plans (id),
  status               text not null default 'active',
  started_at           timestamptz not null default now(),
  ends_at              timestamptz,
  current_period_start date not null,
  current_period_end   date not null,
  created_at           timestamptz not null default now(),
  updated_at           timestamptz not null default now(),
  constraint organization_subscriptions_status_ck check (status in ('trialing', 'active', 'past_due', 'cancelled')),
  constraint organization_subscriptions_period_ck check (current_period_end > current_period_start),
  constraint organization_subscriptions_org_id_uk unique (organization_id, id)
);
create unique index if not exists organization_subscriptions_current_uk
  on subscriptions.organization_subscriptions (organization_id)
  where status in ('trialing', 'active', 'past_due');
create index if not exists organization_subscriptions_plan_idx on subscriptions.organization_subscriptions (plan_id);

create table if not exists subscriptions.usage_counters (
  id              uuid primary key default gen_random_uuid(),
  organization_id uuid not null,
  subscription_id uuid not null,
  metric_code     text not null,
  period_start    date not null,
  period_end      date not null,
  quantity        bigint not null default 0,
  updated_at      timestamptz not null default now(),
  constraint usage_counters_metric_ck check (metric_code in ('documents_issued')),
  constraint usage_counters_quantity_ck check (quantity >= 0),
  constraint usage_counters_period_ck check (period_end > period_start),
  constraint usage_counters_subscription_fk foreign key (organization_id, subscription_id)
    references subscriptions.organization_subscriptions (organization_id, id),
  constraint usage_counters_period_uk unique (organization_id, subscription_id, metric_code, period_start)
);

create or replace trigger plans_set_updated_at                      before update on subscriptions.plans                      for each row execute function shared.set_updated_at();
create or replace trigger organization_subscriptions_set_updated_at before update on subscriptions.organization_subscriptions for each row execute function shared.set_updated_at();
create or replace trigger usage_counters_set_updated_at             before update on subscriptions.usage_counters             for each row execute function shared.set_updated_at();

alter table subscriptions.plans                      enable row level security;
alter table subscriptions.organization_subscriptions enable row level security;
alter table subscriptions.usage_counters             enable row level security;

-- +goose StatementBegin
do $$
declare
  p record;
begin
  for p in
    select * from (values
      ('core', 'roles', 'roles_read',
       $p$create policy roles_read on core.roles for select using (true)$p$),
      ('core', 'organizations', 'organizations_read',
       $p$create policy organizations_read on core.organizations for select
            using (
              id = (select shared.current_organization_id())
              or exists (
                select 1 from core.organization_users ou
                where ou.organization_id = organizations.id
                  and ou.user_id = (select shared.current_user_id())
                  and ou.status = 'active'))$p$),
      ('core', 'organizations', 'organizations_insert',
       $p$create policy organizations_insert on core.organizations for insert with check (true)$p$),
      ('core', 'organizations', 'organizations_update',
       $p$create policy organizations_update on core.organizations for update
            using (id = (select shared.current_organization_id()))
            with check (id = (select shared.current_organization_id()))$p$),
      ('core', 'users', 'users_platform',
       $p$create policy users_platform on core.users for all using (true) with check (true)$p$),
      ('core', 'organization_users', 'organization_users_tenant',
       $p$create policy organization_users_tenant on core.organization_users for all
            using (organization_id = (select shared.current_organization_id()))
            with check (organization_id = (select shared.current_organization_id()))$p$),
      ('core', 'organization_users', 'organization_users_own',
       $p$create policy organization_users_own on core.organization_users for select
            using (user_id = (select shared.current_user_id()))$p$),
      ('core', 'organization_user_roles', 'organization_user_roles_tenant',
       $p$create policy organization_user_roles_tenant on core.organization_user_roles for all
            using (organization_id = (select shared.current_organization_id()))
            with check (organization_id = (select shared.current_organization_id()))$p$),
      ('core', 'organization_user_roles', 'organization_user_roles_own',
       $p$create policy organization_user_roles_own on core.organization_user_roles for select
            using (exists (
              select 1 from core.organization_users ou
              where ou.organization_id = organization_user_roles.organization_id
                and ou.id = organization_user_roles.organization_user_id
                and ou.user_id = (select shared.current_user_id())))$p$),
      ('core', 'invitations', 'invitations_tenant',
       $p$create policy invitations_tenant on core.invitations for all
            using (organization_id = (select shared.current_organization_id()))
            with check (organization_id = (select shared.current_organization_id()))$p$),
      ('core', 'invitations', 'invitations_invitee',
       $p$create policy invitations_invitee on core.invitations for select
            using (lower(email) = (select lower(u.email) from core.users u
                                   where u.id = (select shared.current_user_id())))$p$),
      ('core', 'branches', 'branches_tenant',
       $p$create policy branches_tenant on core.branches for all
            using (organization_id = (select shared.current_organization_id()))
            with check (organization_id = (select shared.current_organization_id()))$p$),
      ('subscriptions', 'plans', 'plans_read',
       $p$create policy plans_read on subscriptions.plans for select using (true)$p$),
      ('subscriptions', 'plans', 'plans_insert',
       $p$create policy plans_insert on subscriptions.plans for insert to platform_app with check (true)$p$),
      ('subscriptions', 'plans', 'plans_update',
       $p$create policy plans_update on subscriptions.plans for update to platform_app using (true) with check (true)$p$),
      ('subscriptions', 'plans', 'plans_delete',
       $p$create policy plans_delete on subscriptions.plans for delete to platform_app using (true)$p$),
      ('subscriptions', 'organization_subscriptions', 'organization_subscriptions_tenant',
       $p$create policy organization_subscriptions_tenant on subscriptions.organization_subscriptions for all
            using (organization_id = (select shared.current_organization_id()))
            with check (organization_id = (select shared.current_organization_id()))$p$),
      ('subscriptions', 'usage_counters', 'usage_counters_tenant',
       $p$create policy usage_counters_tenant on subscriptions.usage_counters for all
            using (organization_id = (select shared.current_organization_id()))
            with check (organization_id = (select shared.current_organization_id()))$p$)
    ) as t(schema_name, table_name, policy_name, ddl)
  loop
    if not exists (
      select 1 from pg_catalog.pg_policies
      where schemaname = p.schema_name and tablename = p.table_name and policyname = p.policy_name
    ) then
      execute p.ddl;
    end if;
  end loop;
end $$;
-- +goose StatementEnd

-- +goose Down
-- La baseline nunca se revierte: estas tablas preexisten al repo y otras APIs dependen de ellas.
select 1;
