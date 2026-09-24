#!/usr/bin/env bash
# Prueba de punta a punta contra la API local y Supabase Auth de DEV.
# Requiere: la API corriendo (go run ./cmd/api), .env y .e2e.local (E2E_EMAIL, E2E_PASSWORD de un usuario de prueba).
# Uso: bash scripts/dev/e2e.sh [http://127.0.0.1:8080]
set -euo pipefail
cd "$(dirname "$0")/../.."

API="${1:-http://127.0.0.1:8080}"
SUPABASE_URL="$(grep -E '^SUPABASE_URL=' .env | cut -d= -f2-)"
PUBLISHABLE_KEY="${SUPABASE_PUBLISHABLE_KEY:-sb_publishable_X0tQqmJnXotXyvec0kCOtw_0h5U2iby}"
# shellcheck disable=SC1091
. ./.e2e.local

pass=0; fail=0
check() { # check "descripción" esperado obtenido
  if [ "$2" = "$3" ]; then pass=$((pass+1)); echo "  ok    $1"; else fail=$((fail+1)); echo "  FALLA $1 (esperado $2, obtenido $3)"; fi
}
json() { python -c "import sys,json;d=json.load(sys.stdin);print(eval('d'+sys.argv[1]))" "$1"; }
claims() { python -c "import sys,json,base64;p=sys.argv[1].split('.')[1];p+='='*(-len(p)%4);print(json.dumps(json.loads(base64.urlsafe_b64decode(p))))" "$1"; }
call() { # call MÉTODO RUTA TOKEN [BODY] [HEADERS...] -> imprime "status|body"
  local method=$1 path=$2 token=$3 body=${4:-}; shift 4 || shift $#
  local args=(-s -o /tmp/e2e_body -w '%{http_code}' -X "$method" "$API$path" -H "Authorization: Bearer $token")
  [ -n "$body" ] && args+=(-H 'Content-Type: application/json' -d "$body")
  for h in "$@"; do args+=(-H "$h"); done
  local code; code=$(curl "${args[@]}")
  echo "$code|$(cat /tmp/e2e_body)"
}
status() { echo "${1%%|*}"; }
body() { echo "${1#*|}"; }

echo "== Login en Supabase Auth ($E2E_EMAIL)"
login=$(curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=password" -H "apikey: $PUBLISHABLE_KEY" \
  -H 'Content-Type: application/json' -d "{\"email\":\"$E2E_EMAIL\",\"password\":\"$E2E_PASSWORD\"}")
TOKEN=$(echo "$login" | json "['access_token']")
REFRESH=$(echo "$login" | json "['refresh_token']")
check "login devuelve token" "True" "$([ -n "$TOKEN" ] && echo True || echo False)"

echo "== /v1/me"
r=$(call GET /v1/me "$TOKEN"); check "GET /v1/me 200" 200 "$(status "$r")"
check "email del token" "$E2E_EMAIL" "$(body "$r" | json "['email']")"

echo "== Organización activa al iniciar sesión"
if [ "$(claims "$TOKEN" | json ".get('org_id')")" = "None" ]; then
  # Primera corrida del usuario: aún no tiene organización.
  r=$(call GET /v1/organizations/current "$TOKEN"); check "GET current sin org → 403" 403 "$(status "$r")"
  check "type no-active-organization" "urn:rdl:platform:problem:no-active-organization" "$(body "$r" | json "['type']")"
else
  # El hook emitió la organización activa que dejó una corrida anterior.
  r=$(call GET /v1/organizations/current "$TOKEN"); check "GET current con la org del login → 200" 200 "$(status "$r")"
  check "coincide con org_id del token" "$(claims "$TOKEN" | json ".get('org_id')")" "$(body "$r" | json "['id']")"
fi

echo "== Alta de organización (idempotente)"
RUN=$(python -c "import secrets;print(secrets.randbelow(10**9))")
KEY="e2e-$RUN"
ORG_BODY="{\"legalName\":\"E2E $RUN S.A.\",\"identificationTypeCode\":\"02\",\"identificationNumber\":\"3101$RUN\",\"email\":\"org$RUN@rdlpymes.com\"}"
r=$(call POST /v1/organizations "$TOKEN" "$ORG_BODY"); check "POST sin Idempotency-Key → 400" 400 "$(status "$r")"
r=$(call POST /v1/organizations "$TOKEN" "$ORG_BODY" "Idempotency-Key: $KEY"); check "POST /v1/organizations 201" 201 "$(status "$r")"
ORG=$(body "$r" | json "['id']")
r=$(call POST /v1/organizations "$TOKEN" "$ORG_BODY" "Idempotency-Key: $KEY"); check "reintento misma clave → 201" 201 "$(status "$r")"
check "reintento devuelve la misma organización" "$ORG" "$(body "$r" | json "['id']")"
r=$(call POST /v1/organizations "$TOKEN" "${ORG_BODY/E2E/Otra}" "Idempotency-Key: $KEY"); check "misma clave, otro body → 422" 422 "$(status "$r")"
r=$(call POST /v1/organizations "$TOKEN" "$ORG_BODY" "Idempotency-Key: $KEY-b"); check "identificación duplicada → 409" 409 "$(status "$r")"

echo "== Selección de organización activa y refresco del token: el hook emite org_id"
# En la primera corrida la organización creada ya queda activa; en las siguientes el usuario conserva la anterior,
# así que se elige explícitamente (flujo del contador con varias organizaciones).
r=$(call PUT /v1/me/active-organization "$TOKEN" "{\"organizationId\":\"$ORG\"}"); check "PUT active-organization 200" 200 "$(status "$r")"
check "pide refrescar el token" "True" "$(body "$r" | json "['tokenRefreshRequired']")"
check "el token vigente aún no trae la organización nueva" "False" "$(claims "$TOKEN" | python -c "import sys,json;print(json.load(sys.stdin).get('org_id')=='$ORG')")"
refreshed=$(curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=refresh_token" -H "apikey: $PUBLISHABLE_KEY" \
  -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$REFRESH\"}")
TOKEN=$(echo "$refreshed" | json "['access_token']")
check "org_id en el token nuevo" "$ORG" "$(claims "$TOKEN" | json ".get('org_id')")"
check "org_roles en el token nuevo" "['owner']" "$(claims "$TOKEN" | json ".get('org_roles')")"

echo "== Organización activa"
r=$(call GET /v1/organizations/current "$TOKEN"); check "GET current 200" 200 "$(status "$r")"
check "es la organización creada" "$ORG" "$(body "$r" | json "['id']")"
r=$(call GET "/v1/organizations/current?organization_id=00000000-0000-0000-0000-000000000001" "$TOKEN" "" "X-Organization-Id: 00000000-0000-0000-0000-000000000001")
check "organization_id en query/header no cambia el tenant" "$ORG" "$(body "$r" | json "['id']")"
r=$(call PATCH /v1/organizations/current "$TOKEN" '{"tradeName":"E2E Comercial","timezone":"America/Panama"}'); check "PATCH current 200" 200 "$(status "$r")"
check "tradeName actualizado" "E2E Comercial" "$(body "$r" | json "['tradeName']")"
r=$(call PATCH /v1/organizations/current "$TOKEN" '{"timezone":"Marte/Olympus"}'); check "zona horaria inválida → 422" 422 "$(status "$r")"

echo "== Membresías"
r=$(call GET /v1/me/memberships "$TOKEN"); check "GET memberships 200" 200 "$(status "$r")"
check "la creada aparece como activa" "True" "$(body "$r" | python -c "import sys,json;d=json.load(sys.stdin);print(any(i['organizationId']=='$ORG' and i['isActive'] and i['roles']==['owner'] for i in d['items']))")"
r=$(call PUT /v1/me/active-organization "$TOKEN" '{"organizationId":"00000000-0000-0000-0000-000000000001"}'); check "activar org ajena → 404" 404 "$(status "$r")"

echo "== Miembros"
r=$(call GET "/v1/organizations/current/users?limit=10" "$TOKEN"); check "GET users 200" 200 "$(status "$r")"
ME=$(body "$r" | json "['items'][0]['userId']")
check "un solo miembro (el creador, owner)" "1 ['owner']" "$(body "$r" | python -c "import sys,json;d=json.load(sys.stdin);print(len(d['items']),d['items'][0]['roles'])")"
r=$(call PATCH "/v1/organizations/current/users/$ME" "$TOKEN" '{"role":"admin"}'); check "degradar al único owner → 409" 409 "$(status "$r")"
check "type last-owner" "urn:rdl:platform:problem:last-owner" "$(body "$r" | json "['type']")"
r=$(call GET "/v1/organizations/current/users?limit=500" "$TOKEN"); check "limit fuera de rango → 422" 422 "$(status "$r")"

echo "== Sucursales"
BR_BODY='{"code":"SJ-01","name":"San José Centro","phone":"2222-2222"}'
r=$(call POST /v1/organizations/current/branches "$TOKEN" "$BR_BODY" "Idempotency-Key: br-$RUN"); check "POST branches 201" 201 "$(status "$r")"
BR_ID=$(body "$r" | json "['id']")
r=$(call POST /v1/organizations/current/branches "$TOKEN" "$BR_BODY" "Idempotency-Key: br-$RUN"); check "reintento: misma sucursal" "$BR_ID" "$(body "$r" | json "['id']")"
r=$(call POST /v1/organizations/current/branches "$TOKEN" '{"code":"SJ-01","name":"Otra"}' "Idempotency-Key: br2-$RUN"); check "código duplicado → 409" 409 "$(status "$r")"
r=$(call POST /v1/organizations/current/branches "$TOKEN" '{"code":"mal código","name":"X"}' "Idempotency-Key: br3-$RUN"); check "código inválido → 422" 422 "$(status "$r")"
r=$(call GET "/v1/organizations/current/branches/$BR_ID" "$TOKEN"); check "GET branch 200" 200 "$(status "$r")"
r=$(call GET "/v1/organizations/current/branches/00000000-0000-0000-0000-000000000001" "$TOKEN"); check "sucursal inexistente → 404" 404 "$(status "$r")"
r=$(call PATCH "/v1/organizations/current/branches/$BR_ID" "$TOKEN" '{"isActive":false,"phone":""}'); check "desactivar sucursal 200" 200 "$(status "$r")"
check "quedó inactiva" "False" "$(body "$r" | json "['isActive']")"
r=$(call GET "/v1/organizations/current/branches?active=true" "$TOKEN"); check "filtro active=true la excluye" 0 "$(body "$r" | json "['items'].__len__()")"
r=$(call GET "/v1/organizations/current/branches?active=false" "$TOKEN"); check "filtro active=false la incluye" 1 "$(body "$r" | json "['items'].__len__()")"

if [ -n "${E2E_EMAIL2:-}" ]; then
  echo "== Invitaciones (segundo usuario: $E2E_EMAIL2)"
  INV_KEY="inv-$RUN"
  r=$(call POST /v1/organizations/current/invitations "$TOKEN" "{\"email\":\"$E2E_EMAIL2\",\"role\":\"biller\"}" "Idempotency-Key: $INV_KEY")
  check "POST invitations 201" 201 "$(status "$r")"
  INV_TOKEN=$(body "$r" | json "['token']"); INV_ID=$(body "$r" | json "['id']")
  r=$(call POST /v1/organizations/current/invitations "$TOKEN" "{\"email\":\"$E2E_EMAIL2\",\"role\":\"biller\"}" "Idempotency-Key: $INV_KEY")
  check "reintento: misma invitación" "$INV_ID" "$(body "$r" | json "['id']")"
  check "reintento: sin token (solo se guarda el hash)" "None" "$(body "$r" | json ".get('token')")"
  r=$(call GET "/v1/organizations/current/invitations?status=pending" "$TOKEN"); check "GET invitations 200" 200 "$(status "$r")"
  check "la invitación aparece pendiente" "True" "$(body "$r" | python -c "import sys,json;print(any(i['id']=='$INV_ID' and i['status']=='pending' for i in json.load(sys.stdin)['items']))")"

  r=$(call POST "/v1/invitations/$INV_TOKEN/accept" "$TOKEN" "" "Idempotency-Key: robo-$RUN"); check "otro usuario no puede aceptarla → 404" 404 "$(status "$r")"

  login2=$(curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=password" -H "apikey: $PUBLISHABLE_KEY" \
    -H 'Content-Type: application/json' -d "{\"email\":\"$E2E_EMAIL2\",\"password\":\"$E2E_PASSWORD2\"}")
  TOKEN2=$(echo "$login2" | json "['access_token']"); REFRESH2=$(echo "$login2" | json "['refresh_token']")
  r=$(call POST "/v1/invitations/$INV_TOKEN/accept" "$TOKEN2" "" "Idempotency-Key: acc-$RUN"); check "aceptar invitación 200" 200 "$(status "$r")"
  check "rol invitado" "biller" "$(body "$r" | json "['role']")"
  r=$(call POST "/v1/invitations/$INV_TOKEN/accept" "$TOKEN2" "" "Idempotency-Key: acc-$RUN"); check "reintento misma clave → 200" 200 "$(status "$r")"
  r=$(call POST "/v1/invitations/$INV_TOKEN/accept" "$TOKEN2" "" "Idempotency-Key: acc2-$RUN"); check "segundo uso → 409" 409 "$(status "$r")"

  r=$(call PUT /v1/me/active-organization "$TOKEN2" "{\"organizationId\":\"$ORG\"}"); check "usuario 2 elige la organización" 200 "$(status "$r")"
  TOKEN2=$(curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=refresh_token" -H "apikey: $PUBLISHABLE_KEY" \
    -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$REFRESH2\"}" | json "['access_token']")
  check "token del usuario 2 con org_roles biller" "['biller']" "$(claims "$TOKEN2" | json ".get('org_roles')")"

  echo "== Permisos por rol (usuario 2 = facturador)"
  r=$(call GET /v1/organizations/current "$TOKEN2"); check "facturador lee la organización" 200 "$(status "$r")"
  r=$(call PATCH /v1/organizations/current "$TOKEN2" '{"tradeName":"hack"}'); check "facturador no edita la organización → 403" 403 "$(status "$r")"
  r=$(call GET /v1/organizations/current/users "$TOKEN2"); check "facturador no lista miembros → 403" 403 "$(status "$r")"
  r=$(call GET "/v1/organizations/current/branches/$BR_ID" "$TOKEN2"); check "facturador lee sucursales" 200 "$(status "$r")"
  r=$(call POST /v1/organizations/current/branches "$TOKEN2" '{"code":"FAC","name":"X"}' "Idempotency-Key: brf-$RUN"); check "facturador no crea sucursales → 403" 403 "$(status "$r")"
  r=$(call PATCH "/v1/organizations/current/branches/$BR_ID" "$TOKEN2" '{"isActive":true}'); check "facturador no edita sucursales → 403" 403 "$(status "$r")"

  echo "== Gestión de miembros"
  r=$(call GET /v1/organizations/current/users "$TOKEN"); check "ahora hay 2 miembros" 2 "$(body "$r" | json "['items'].__len__()")"
  U2=$(body "$r" | python -c "import sys,json;print([i['userId'] for i in json.load(sys.stdin)['items'] if i['email']=='$E2E_EMAIL2'][0])")
  r=$(call PATCH "/v1/organizations/current/users/$U2" "$TOKEN" '{"role":"admin"}'); check "owner asciende al usuario 2 a admin" 200 "$(status "$r")"
  r=$(call GET /v1/organizations/current/users "$TOKEN2")
  check "el nuevo rol rige de inmediato (la BD manda, no el JWT viejo)" 200 "$(status "$r")"
  r=$(call PATCH "/v1/organizations/current/users/$ME" "$TOKEN2" '{"role":"biller"}'); check "admin no puede degradar a un owner → 403" 403 "$(status "$r")"
  check "type owner-required" "urn:rdl:platform:problem:owner-required" "$(body "$r" | json "['type']")"
  r=$(call PATCH "/v1/organizations/current/users/$U2" "$TOKEN" '{"status":"suspended"}'); check "owner suspende al usuario 2" 200 "$(status "$r")"
  r=$(call GET /v1/organizations/current "$TOKEN2"); check "usuario suspendido pierde acceso de inmediato → 403" 403 "$(status "$r")"
  check "type membership-inactive" "urn:rdl:platform:problem:membership-inactive" "$(body "$r" | json "['type']")"

  echo "== Revocar invitación"
  r=$(call POST /v1/organizations/current/invitations "$TOKEN" "{\"email\":\"revocar-$RUN@rdlpymes.com\",\"role\":\"read_only\"}" "Idempotency-Key: rev-$RUN")
  REV_ID=$(body "$r" | json "['id']"); REV_TOKEN=$(body "$r" | json "['token']")
  r=$(call DELETE "/v1/organizations/current/invitations/$REV_ID" "$TOKEN"); check "DELETE invitation 204" 204 "$(status "$r")"
  r=$(call DELETE "/v1/organizations/current/invitations/$REV_ID" "$TOKEN"); check "DELETE repetido 204" 204 "$(status "$r")"
fi

echo "== Token forjado"
FORGED="$(echo "$TOKEN" | cut -d. -f1).$(python -c "import base64,json,sys;print(base64.urlsafe_b64encode(json.dumps({'sub':'x','aud':'authenticated','exp':9999999999,'org_id':'$ORG'}).encode()).rstrip(b'=').decode())").$(echo "$TOKEN" | cut -d. -f3)"
r=$(call GET /v1/organizations/current "$FORGED"); check "payload alterado → 401" 401 "$(status "$r")"

echo
echo "Resultado: $pass ok, $fail fallas (organización $ORG)"
[ "$fail" -eq 0 ]
