-- Configuración fiscal (incremento 4). Todo bajo RLS de la organización de la sesión; la organización también en el
-- WHERE o en el INSERT (defensa en profundidad). Los secretos nunca pasan por aquí: solo sus referencias.

-- name: UpsertTaxpayerProfile :one
insert into fiscal.taxpayer_profiles (
  organization_id, identification_type_code, identification_number, legal_name, trade_name, email, phone,
  province_code, canton_code, district_code, address_details, environment
) values (
  sqlc.arg(organization_id), sqlc.arg(identification_type_code), sqlc.arg(identification_number),
  sqlc.arg(legal_name), sqlc.narg(trade_name), sqlc.arg(email), sqlc.narg(phone), sqlc.narg(province_code),
  sqlc.narg(canton_code), sqlc.narg(district_code), sqlc.narg(address_details), sqlc.arg(environment)
)
on conflict (organization_id) do update set
  identification_type_code = excluded.identification_type_code,
  identification_number    = excluded.identification_number,
  legal_name               = excluded.legal_name,
  trade_name               = excluded.trade_name,
  email                    = excluded.email,
  phone                    = excluded.phone,
  province_code            = excluded.province_code,
  canton_code              = excluded.canton_code,
  district_code            = excluded.district_code,
  address_details          = excluded.address_details,
  environment              = excluded.environment
returning (xmax = 0)::boolean as created;

-- Una sola actividad principal (índice único parcial de la migración 00002): primero se baja la anterior.
-- name: ClearPrimaryActivity :exec
update fiscal.organization_economic_activities set is_primary = false
 where organization_id = sqlc.arg(organization_id) and is_primary;

-- name: SetPrimaryActivity :exec
insert into fiscal.organization_economic_activities (organization_id, activity_code, is_primary)
values (sqlc.arg(organization_id), sqlc.arg(activity_code), true)
on conflict (organization_id, activity_code) do update set is_primary = true;

-- name: IdentificationTypeExists :one
select exists (select 1 from fiscal.identification_types where code = sqlc.arg(code) and is_active)::boolean;

-- name: DistrictExists :one
select exists (
  select 1 from fiscal.districts
   where province_code = sqlc.arg(province_code) and canton_code = sqlc.arg(canton_code) and code = sqlc.arg(district_code)
)::boolean;

-- name: EconomicActivityExists :one
select exists (select 1 from fiscal.economic_activities where code = sqlc.arg(code) and is_active)::boolean;

-- Devuelve las referencias anteriores para borrarlas del SecretStore después del commit.
-- name: SetHaciendaCredentialRefs :one
with prev as (
  select hacienda_username_secret_id, hacienda_password_secret_id
    from fiscal.taxpayer_profiles where organization_id = sqlc.arg(organization_id) for update
)
update fiscal.taxpayer_profiles p
   set hacienda_username_secret_id = sqlc.arg(username_secret_id),
       hacienda_password_secret_id = sqlc.arg(password_secret_id)
  from prev
 where p.organization_id = sqlc.arg(organization_id)
returning prev.hacienda_username_secret_id as prev_username_secret_id,
          prev.hacienda_password_secret_id as prev_password_secret_id;

-- name: DeactivateCertificates :exec
update fiscal.certificates set is_active = false
 where organization_id = sqlc.arg(organization_id) and is_active;

-- name: InsertCertificate :exec
insert into fiscal.certificates (
  id, organization_id, storage_path, pin_secret_id, subject, issuer, serial_number, valid_from, valid_to,
  is_active, uploaded_by_user_id
) values (
  sqlc.arg(id), sqlc.arg(organization_id), sqlc.arg(storage_path), sqlc.arg(pin_secret_id), sqlc.arg(subject),
  sqlc.arg(issuer), sqlc.arg(serial_number), sqlc.arg(valid_from), sqlc.arg(valid_to), true,
  sqlc.arg(uploaded_by_user_id)
);
