-- Fixtures de la suite de aislamiento de Billing (tests/isolation). SOLO DEV.
-- Ejecutar como postgres desde el SQL Editor de Supabase. Es idempotente: se puede correr varias veces.
-- Billing no escribe en core (solo lee): por eso estos datos los crea este script y no la suite.
--
-- Dos organizaciones de prueba con ids fijos (la suite los conoce):
--   A (b0000000-...000a): owner activo, lector (read_only) activo, facturador SUSPENDIDO; sucursal ISO-A.
--   B (b0000000-...000b): owner activo; sucursal ISO-B.
-- Los usuarios no existen en auth.users: la suite verifica los JWT con un verificador de prueba, pero la membresía
-- se revalida contra estas filas reales, como en producción.

begin;

insert into core.organizations (id, legal_name, identification_type_code, identification_number, email)
values ('b0000000-0000-4000-8000-00000000000a', 'Aislamiento Billing A (prueba)', '02', '3101999991', 'billing-iso-a@rdlpymes.invalid'),
       ('b0000000-0000-4000-8000-00000000000b', 'Aislamiento Billing B (prueba)', '02', '3101999992', 'billing-iso-b@rdlpymes.invalid')
on conflict (id) do nothing;

insert into core.users (id, identity_provider, external_subject, email, full_name, active_organization_id)
values ('b0000000-0000-4000-8000-0000000000a1', 'supabase', 'billing-iso-owner-a',     'billing-iso-owner-a@rdlpymes.invalid',     'Owner A',      'b0000000-0000-4000-8000-00000000000a'),
       ('b0000000-0000-4000-8000-0000000000a2', 'supabase', 'billing-iso-reader-a',    'billing-iso-reader-a@rdlpymes.invalid',    'Lector A',     'b0000000-0000-4000-8000-00000000000a'),
       ('b0000000-0000-4000-8000-0000000000a3', 'supabase', 'billing-iso-suspended-a', 'billing-iso-suspended-a@rdlpymes.invalid', 'Suspendido A', 'b0000000-0000-4000-8000-00000000000a'),
       ('b0000000-0000-4000-8000-0000000000b1', 'supabase', 'billing-iso-owner-b',     'billing-iso-owner-b@rdlpymes.invalid',     'Owner B',      'b0000000-0000-4000-8000-00000000000b')
on conflict (id) do nothing;

insert into core.organization_users (id, organization_id, user_id, status)
values ('b0000000-0000-4000-8000-0000000001a1', 'b0000000-0000-4000-8000-00000000000a', 'b0000000-0000-4000-8000-0000000000a1', 'active'),
       ('b0000000-0000-4000-8000-0000000001a2', 'b0000000-0000-4000-8000-00000000000a', 'b0000000-0000-4000-8000-0000000000a2', 'active'),
       ('b0000000-0000-4000-8000-0000000001a3', 'b0000000-0000-4000-8000-00000000000a', 'b0000000-0000-4000-8000-0000000000a3', 'suspended'),
       ('b0000000-0000-4000-8000-0000000001b1', 'b0000000-0000-4000-8000-00000000000b', 'b0000000-0000-4000-8000-0000000000b1', 'active')
on conflict (id) do nothing;

insert into core.organization_user_roles (organization_id, organization_user_id, role_code)
values ('b0000000-0000-4000-8000-00000000000a', 'b0000000-0000-4000-8000-0000000001a1', 'owner'),
       ('b0000000-0000-4000-8000-00000000000a', 'b0000000-0000-4000-8000-0000000001a2', 'read_only'),
       ('b0000000-0000-4000-8000-00000000000a', 'b0000000-0000-4000-8000-0000000001a3', 'biller'),
       ('b0000000-0000-4000-8000-00000000000b', 'b0000000-0000-4000-8000-0000000001b1', 'owner')
on conflict do nothing;

insert into core.branches (id, organization_id, code, name)
values ('b0000000-0000-4000-8000-00000000ba01', 'b0000000-0000-4000-8000-00000000000a', 'ISO-A', 'Sucursal A (prueba)'),
       ('b0000000-0000-4000-8000-00000000bb01', 'b0000000-0000-4000-8000-00000000000b', 'ISO-B', 'Sucursal B (prueba)')
on conflict (id) do nothing;

commit;
