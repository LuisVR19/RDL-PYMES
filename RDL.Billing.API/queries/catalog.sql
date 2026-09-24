-- Lecturas de otros schemas que Billing necesita para armar un documento. Solo SELECT (docs/decisiones/0002).

-- La organización de la sesión (política organizations_read): moneda local y zona horaria de las fechas de negocio.
-- name: GetOrganizationSettings :one
select default_currency_code::text as default_currency_code, timezone
from core.organizations where id = sqlc.arg(organization_id) and status = 'active';

-- Un branch_id recibido se valida contra la organización activa (política branches_tenant).
-- name: GetBranch :one
select id, is_active from core.branches where organization_id = $1 and id = $2;

-- Tarifas vigentes por código. El porcentaje sale del catálogo, nunca del cliente (informe 0001 §4.2).
-- name: ListActiveTaxRates :many
select code, rate::text as rate from fiscal.tax_rates where is_active and code = any(sqlc.arg(codes)::text[]);
