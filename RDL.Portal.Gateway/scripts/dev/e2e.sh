#!/usr/bin/env bash
# Prueba de punta a punta del Portal Gateway corriendo, contra las APIs reales en local y Supabase Auth de DEV.
# Requiere: el gateway corriendo (make run), Platform corriendo (:8080) y .e2e.local con E2E_EMAIL/E2E_PASSWORD
# de un usuario de prueba (el mismo del E2E de Platform, que ya tiene organización activa). Billing es opcional:
# si no responde, sus casos comprueban la degradación en vez del dato.
# Uso: bash scripts/dev/e2e.sh [http://127.0.0.1:8090]
set -euo pipefail
cd "$(dirname "$0")/../.."

GW="${1:-http://127.0.0.1:8090}"
SUPABASE_URL="${SUPABASE_URL:-https://dzlsnsstuqpxvwegeqcy.supabase.co}"
PUBLISHABLE_KEY="${SUPABASE_PUBLISHABLE_KEY:-sb_publishable_X0tQqmJnXotXyvec0kCOtw_0h5U2iby}"
BILLING_API_URL="${BILLING_API_URL:-http://localhost:8081}"
# shellcheck disable=SC1091
[ -f ./.e2e.local ] && . ./.e2e.local
: "${E2E_EMAIL:?falta E2E_EMAIL (entorno o .e2e.local)}"
: "${E2E_PASSWORD:?falta E2E_PASSWORD (entorno o .e2e.local)}"

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
pass=0; fail=0
check() { # check "descripción" esperado obtenido
  if [ "$2" = "$3" ]; then pass=$((pass+1)); echo "  ok    $1"; else fail=$((fail+1)); echo "  FALLA $1 (esperado $2, obtenido $3)"; fi
}
json() { python -c "import sys,json;d=json.load(sys.stdin);print(eval('d'+sys.argv[1]))" "$1"; }
claims() { python -c "import sys,json,base64;p=sys.argv[1].split('.')[1];p+='='*(-len(p)%4);print(json.dumps(json.loads(base64.urlsafe_b64decode(p))))" "$1"; }
call() { # call MÉTODO RUTA TOKEN [BODY] [HEADERS...] -> "status|body"; headers de respuesta en $TMP/headers
  local method=$1 path=$2 token=$3 body=${4:-}; shift 4 || shift $#
  local args=(-s -o "$TMP/body" -D "$TMP/headers" -w '%{http_code}' -X "$method" "$GW$path")
  [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
  [ -n "$body" ] && args+=(-H 'Content-Type: application/json' -d "$body")
  for h in "$@"; do args+=(-H "$h"); done
  local code; code=$(curl "${args[@]}")
  echo "$code|$(cat "$TMP/body")"
}
status() { echo "${1%%|*}"; }
body() { echo "${1#*|}"; }
header() { grep -i "^$1:" "$TMP/headers" | head -1 | cut -d: -f2- | tr -d ' \r'; }
ptype() { body "$1" | json ".get('type')"; }
P=urn:rdl:portal-gateway:problem

echo "== Salud"
r=$(call GET /healthz ""); check "/healthz 200" 200 "$(status "$r")"
r=$(call GET /readyz ""); check "/readyz 200 (JWKS arriba)" 200 "$(status "$r")"

echo "== Sin sesión"
r=$(call GET /portal/v1/me ""); check "sin token → 401" 401 "$(status "$r")"
check "type unauthenticated" "$P:unauthenticated" "$(ptype "$r")"
r=$(call GET /portal/v1/me "no.es.un.jwt"); check "token basura → 401" 401 "$(status "$r")"

echo "== Login en Supabase Auth ($E2E_EMAIL)"
login=$(curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=password" -H "apikey: $PUBLISHABLE_KEY" \
  -H 'Content-Type: application/json' -d "{\"email\":\"$E2E_EMAIL\",\"password\":\"$E2E_PASSWORD\"}")
TOKEN=$(echo "$login" | json ".get('access_token','')")
check "login devuelve token" "True" "$([ -n "$TOKEN" ] && echo True || echo False)"
ORG=$(claims "$TOKEN" | json ".get('org_id')")
if [ "$ORG" = "None" ]; then
  echo "El usuario no tiene organización activa: corra primero el E2E de Platform (crea y activa una)."; exit 1
fi

echo "== Paso directo a Platform"
r=$(call GET /portal/v1/me "$TOKEN"); check "GET /portal/v1/me 200" 200 "$(status "$r")"
check "email del token" "$E2E_EMAIL" "$(body "$r" | json "['email']")"
check "genera X-Correlation-Id" "True" "$([ -n "$(header X-Correlation-Id)" ] && echo True || echo False)"
CID="$(python -c "import uuid;print(uuid.uuid4())")"
r=$(call GET /portal/v1/organizations/current "$TOKEN" "" "X-Correlation-Id: $CID")
check "GET organizations/current 200" 200 "$(status "$r")"
check "es la organización del token" "$ORG" "$(body "$r" | json "['id']")"
check "respeta el X-Correlation-Id (UUID) del cliente" "$CID" "$(header X-Correlation-Id)"
r=$(call GET /portal/v1/me "$TOKEN" "" "X-Correlation-Id: no-es-uuid")
check "un X-Correlation-Id que no es UUID se reemplaza (convenciones)" "True" "$([ "$(header X-Correlation-Id)" != "no-es-uuid" ] && echo True || echo False)"
r=$(call GET "/portal/v1/organizations/current?organization_id=00000000-0000-0000-0000-000000000001" "$TOKEN" "" \
  "X-Organization-Id: 00000000-0000-0000-0000-000000000001")
check "organización en query/header no cambia el tenant" "$ORG" "$(body "$r" | json "['id']")"
r=$(call PUT /portal/v1/me/active-organization "$TOKEN" '{"organizationId":"00000000-0000-0000-0000-000000000001"}')
check "activar organización ajena → 404 de Platform" 404 "$(status "$r")"
check "el problema es de Platform, sin reescribir" "True" "$(ptype "$r" | grep -q '^urn:rdl:platform:problem:' && echo True || echo False)"
r=$(call GET /portal/v1/me/memberships "$TOKEN"); check "GET memberships 200" 200 "$(status "$r")"

echo "== Superficie cerrada"
r=$(call GET /portal/v1/no-existe "$TOKEN"); check "ruta no declarada → 404" 404 "$(status "$r")"
check "type not-found del gateway" "$P:not-found" "$(ptype "$r")"
r=$(call GET /internal/v1/invoices/00000000-0000-0000-0000-000000000001/summary "$TOKEN"); check "/internal jamás se expone → 404" 404 "$(status "$r")"
r=$(call GET /v1/me "$TOKEN"); check "ruta de la API sin /portal → 404" 404 "$(status "$r")"
r=$(call DELETE /portal/v1/me "$TOKEN"); check "método no declarado → 405" 405 "$(status "$r")"
r=$(call GET /portal/v1/me "$TOKEN"); check "sin header Server de la API" "" "$(header Server)"

echo "== APIs no desplegadas (E-Invoice, Receivables)"
r=$(call GET /portal/v1/fiscal-profile "$TOKEN"); check "fiscal sin URL → 503" 503 "$(status "$r")"
check "type upstream-not-configured" "$P:upstream-not-configured" "$(ptype "$r")"

echo "== Billing y vista transversal"
NOPE=00000000-0000-0000-0000-000000000001
if curl -s -m 3 -o /dev/null "$BILLING_API_URL/healthz"; then
  r=$(call GET /portal/v1/customers "$TOKEN"); check "GET customers 200" 200 "$(status "$r")"
  r=$(call GET "/portal/v1/invoices/$NOPE/overview" "$TOKEN"); check "overview de factura inexistente → 404" 404 "$(status "$r")"
  r=$(call GET "/portal/v1/invoices?limit=5" "$TOKEN"); check "listado compuesto 200" 200 "$(status "$r")"
  check "listado con nextCursor" "True" "$(body "$r" | json ".__contains__('nextCursor')")"
  r=$(call GET "/portal/v1/invoices?limit=500" "$TOKEN"); check "limit fuera de rango → 422 de Billing" 422 "$(status "$r")"
else
  echo "  (Billing no responde en $BILLING_API_URL: se comprueba la degradación)"
  r=$(call GET /portal/v1/customers "$TOKEN"); check "Billing caída → 502" 502 "$(status "$r")"
  check "type upstream-unavailable" "$P:upstream-unavailable" "$(ptype "$r")"
  r=$(call GET "/portal/v1/invoices/$NOPE/overview" "$TOKEN"); check "overview sin Billing → 502" 502 "$(status "$r")"
  r=$(call GET "/portal/v1/invoices" "$TOKEN"); check "listado sin Billing → 502" 502 "$(status "$r")"
fi

echo "== CORS"
code=$(curl -s -o /dev/null -D "$TMP/headers" -w '%{http_code}' -X OPTIONS "$GW/portal/v1/me" \
  -H 'Origin: http://localhost:5173' -H 'Access-Control-Request-Method: GET' -H 'Access-Control-Request-Headers: authorization')
check "preflight del portal sin token → 204" 204 "$code"
check "permite el origen del portal" "http://localhost:5173" "$(header Access-Control-Allow-Origin)"
curl -s -o /dev/null -D "$TMP/headers" -X OPTIONS "$GW/portal/v1/me" \
  -H 'Origin: https://evil.example' -H 'Access-Control-Request-Method: GET'
check "origen desconocido sin Allow-Origin" "" "$(header Access-Control-Allow-Origin)"

echo
echo "Resultado: $pass ok, $fail fallas"
[ "$fail" -eq 0 ]
