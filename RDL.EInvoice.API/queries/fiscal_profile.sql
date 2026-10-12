-- Perfil fiscal de la organización de la sesión (RLS taxpayer_profiles_tenant). La organización también va en el
-- WHERE: defensa en profundidad, igual que en el resto de consultas. A lo sumo una actividad principal y un
-- certificado activo por organización (índices únicos parciales de la migración 00002).

-- name: GetTaxpayerProfile :one
select p.identification_type_code, p.identification_number, p.legal_name, p.trade_name, p.email, p.phone,
       p.province_code, p.canton_code, p.district_code, p.address_details, p.environment,
       (p.hacienda_username_secret_id is not null and p.hacienda_password_secret_id is not null)::boolean
         as hacienda_credentials_set,
       p.updated_at,
       a.activity_code as primary_activity_code,
       c.valid_to      as certificate_valid_to
from fiscal.taxpayer_profiles p
left join fiscal.organization_economic_activities a
  on a.organization_id = p.organization_id and a.is_primary
left join fiscal.certificates c
  on c.organization_id = p.organization_id and c.is_active
where p.organization_id = sqlc.arg(organization_id);
