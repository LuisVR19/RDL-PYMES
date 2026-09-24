#!/usr/bin/env bash
# Prueba de punta a punta de Billing contra la API local y Supabase Auth de DEV.
#
# Requiere:
#   - Billing corriendo (make run, :8081) con su .env;
#   - .e2e.local en la raíz de Billing con E2E_EMAIL y E2E_PASSWORD de un usuario de prueba que ya tenga una
#     organización activa (se crea con el E2E de Platform) y, opcional, E2E_EMAIL2/E2E_PASSWORD2 de otro usuario de
#     otra organización o sin ella, para la parte de aislamiento. .e2e.local está ignorado por git.
#
# ATENCIÓN: a diferencia de las suites de Go, este script CONFIRMA datos en la base dev: un cliente, un producto, una
# factura emitida, su audit y su InvoiceIssued en el outbox (que el worker publicará). Es lo que pide el E2E.
#
# Uso: bash scripts/dev/e2e.sh [http://127.0.0.1:8081]
set -euo pipefail
cd "$(dirname "$0")/../.."

API="${1:-http://127.0.0.1:8081}"
SUPABASE_URL="$(grep -E '^SUPABASE_URL=' .env | cut -d= -f2-)"
PUBLISHABLE_KEY="${SUPABASE_PUBLISHABLE_KEY:-sb_publishable_X0tQqmJnXotXyvec0kCOtw_0h5U2iby}"
# shellcheck disable=SC1091
. ./.e2e.local

pass=0; fail=0
check() { # check "descripción" esperado obtenido
  if [ "$2" = "$3" ]; then pass=$((pass+1)); echo "  ok    $1"; else fail=$((fail+1)); echo "  FALLA $1 (esperado $2, obtenido $3)"; fi
}
json() { python3 -c "import sys,json;d=json.load(sys.stdin);print(eval('d'+sys.argv[1]))" "$1"; }
claims() { python3 -c "import sys,json,base64;p=sys.argv[1].split('.')[1];p+='='*(-len(p)%4);print(json.dumps(json.loads(base64.urlsafe_b64decode(p))))" "$1"; }
call() { # call MÉTODO RUTA TOKEN [BODY] [HEADERS...] -> imprime "status|body"
  local method=$1 path=$2 token=$3 body=${4:-}; shift 4 || shift $#
  local args=(-s -o /tmp/billing_e2e_body -w '%{http_code}' -X "$method" "$API$path" -H "Authorization: Bearer $token")
  [ -n "$body" ] && args+=(-H 'Content-Type: application/json' -d "$body")
  for h in "$@"; do args+=(-H "$h"); done
  local code; code=$(curl "${args[@]}")
  echo "$code|$(cat /tmp/billing_e2e_body)"
}
status() { echo "${1%%|*}"; }
body() { echo "${1#*|}"; }
login() { # login EMAIL PASSWORD -> access token
  curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=password" -H "apikey: $PUBLISHABLE_KEY" \
    -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"$2\"}" | json "['access_token']"
}

echo "== Login en Supabase Auth ($E2E_EMAIL)"
TOKEN=$(login "$E2E_EMAIL" "$E2E_PASSWORD")
ORG=$(claims "$TOKEN" | json ".get('org_id')")
if [ "$ORG" = "None" ]; then
  echo "El usuario no tiene organización activa: corra primero el E2E de Platform (crea y activa una)."; exit 1
fi
check "token con org_id" "True" "$([ -n "$ORG" ] && echo True || echo False)"

RUN=$(python3 -c "import secrets;print(secrets.randbelow(10**6))")

echo "== Clientes"
r=$(call GET /v1/customers "$TOKEN"); check "GET /v1/customers 200" 200 "$(status "$r")"
C_BODY="{\"identification\":{\"typeCode\":\"02\",\"number\":\"3101$RUN\"},\"legalName\":\"Cliente E2E $RUN S.A.\",\"email\":\"cliente$RUN@rdlpymes.invalid\"}"
r=$(call POST /v1/customers "$TOKEN" "$C_BODY"); check "sin Idempotency-Key → 400" 400 "$(status "$r")"
r=$(call POST /v1/customers "$TOKEN" "$C_BODY" "Idempotency-Key: c-$RUN"); check "POST cliente 201" 201 "$(status "$r")"
CUSTOMER=$(body "$r" | json "['id']")
r=$(call POST /v1/customers "$TOKEN" "$C_BODY" "Idempotency-Key: c-$RUN"); check "reintento: mismo cliente" "$CUSTOMER" "$(body "$r" | json "['id']")"
r=$(call POST /v1/customers "$TOKEN" "$C_BODY" "Idempotency-Key: c2-$RUN"); check "identificación repetida → 409" 409 "$(status "$r")"

echo "== Productos (sin impuestos: fiscal.tax_rates está vacío en dev)"
P_BODY="{\"code\":\"E2E-$RUN\",\"description\":\"Servicio E2E\",\"cabysCode\":\"8314100000100\",\"unitOfMeasureCode\":\"Sp\",\"unitPrice\":\"0.33333\",\"currency\":\"CRC\",\"isService\":true}"
r=$(call POST /v1/products "$TOKEN" "$P_BODY" "Idempotency-Key: p-$RUN"); check "POST producto 201" 201 "$(status "$r")"
PRODUCT=$(body "$r" | json "['id']")
r=$(call POST /v1/products "$TOKEN" "${P_BODY/0.33333/25000}" "Idempotency-Key: p2-$RUN"); check "código de producto repetido → 409" 409 "$(status "$r")"

echo "== Borrador"
D_BODY="{\"documentType\":\"invoice\",\"customerId\":\"$CUSTOMER\",\"saleConditionCode\":\"01\",\"currency\":\"CRC\",\"lines\":[{\"productId\":\"$PRODUCT\",\"quantity\":\"1\"}]}"
r=$(call POST /v1/invoices "$TOKEN" "$D_BODY" "Idempotency-Key: d-$RUN"); check "POST borrador 201" 201 "$(status "$r")"
INV=$(body "$r" | json "['id']")
check "número null en borrador" "None" "$(body "$r" | json "['number']")"
r=$(call PUT "/v1/invoices/$INV/lines" "$TOKEN" "[{\"productId\":\"$PRODUCT\",\"quantity\":\"3\"}]"); check "PUT líneas 200" 200 "$(status "$r")"
check "total redondeado (3 × 0.33333)" "0.99999" "$(body "$r" | json "['total']")"
r=$(call POST /v1/invoices/$INV/issue "$TOKEN" ""); check "emitir sin Idempotency-Key → 400" 400 "$(status "$r")"

echo "== Emisión"
r=$(call POST "/v1/invoices/$INV/issue" "$TOKEN" "" "Idempotency-Key: i-$RUN"); check "emitir 201" 201 "$(status "$r")"
NUMBER=$(body "$r" | json "['number']")
check "estado issued" "issued" "$(body "$r" | json "['status']")"
check "snapshot del cliente" "Cliente E2E $RUN S.A." "$(body "$r" | json "['customerSnapshot']['legalName']")"
r=$(call POST "/v1/invoices/$INV/issue" "$TOKEN" "" "Idempotency-Key: i-$RUN"); check "reintento misma clave → 201" 201 "$(status "$r")"
check "reintento: mismo número" "$NUMBER" "$(body "$r" | json "['number']")"
r=$(call POST "/v1/invoices/$INV/issue" "$TOKEN" "" "Idempotency-Key: i2-$RUN"); check "emitir de nuevo → 409" 409 "$(status "$r")"
check "type invoice-not-draft" "urn:rdl:billing:problem:invoice-not-draft" "$(body "$r" | json "['type']")"
r=$(call PATCH "/v1/invoices/$INV" "$TOKEN" '{"notes":"hack"}'); check "editar emitida → 409" 409 "$(status "$r")"
r=$(call DELETE "/v1/invoices/$INV" "$TOKEN"); check "descartar emitida → 409" 409 "$(status "$r")"
r=$(call GET "/v1/invoices/$INV/history" "$TOKEN"); check "historial draft → issued" "issued" "$(body "$r" | python3 -c "import sys,json;print(json.load(sys.stdin)[0]['toStatus'])")"

echo "== Outbox"
check "InvoiceIssued en el outbox y válido" "InvoiceIssued valido" "$(go run ./scripts/dev/outboxcheck -org "$ORG" -invoice "$INV" 2>&1 | tail -1)"

echo "== Snapshots (criterio 5)"
r=$(call PATCH "/v1/customers/$CUSTOMER" "$TOKEN" '{"legalName":"Cambiado"}'); check "editar cliente 200" 200 "$(status "$r")"
r=$(call PATCH "/v1/products/$PRODUCT" "$TOKEN" '{"unitPrice":"99999"}'); check "editar producto 200" 200 "$(status "$r")"
r=$(call GET "/v1/invoices/$INV" "$TOKEN")
check "la factura conserva el cliente" "Cliente E2E $RUN S.A." "$(body "$r" | json "['customerSnapshot']['legalName']")"
check "la factura conserva el precio" "0.33333" "$(body "$r" | json "['lines'][0]['unitPrice']")"

echo "== Tenant solo del token"
r=$(call GET "/v1/invoices/$INV?organizationId=00000000-0000-0000-0000-000000000001" "$TOKEN" "" "X-Organization-Id: 00000000-0000-0000-0000-000000000001")
check "query/header no cambian el tenant" 200 "$(status "$r")"

if [ -n "${E2E_EMAIL2:-}" ]; then
  echo "== Aislamiento (segundo usuario: $E2E_EMAIL2)"
  TOKEN2=$(login "$E2E_EMAIL2" "$E2E_PASSWORD2")
  r=$(call GET "/v1/invoices/$INV" "$TOKEN2"); s=$(status "$r")
  check "otro usuario no ve la factura (403/404)" "True" "$([ "$s" = 403 ] || [ "$s" = 404 ] && echo True || echo False)"
  r=$(call PATCH "/v1/customers/$CUSTOMER" "$TOKEN2" '{"legalName":"Robado"}'); s=$(status "$r")
  check "otro usuario no edita el cliente (403/404)" "True" "$([ "$s" = 403 ] || [ "$s" = 404 ] && echo True || echo False)"
fi

echo "== Token forjado"
FORGED="$(echo "$TOKEN" | cut -d. -f1).$(python3 -c "import base64,json;print(base64.urlsafe_b64encode(json.dumps({'sub':'x','aud':'authenticated','exp':9999999999,'org_id':'$ORG'}).encode()).rstrip(b'=').decode())").$(echo "$TOKEN" | cut -d. -f3)"
r=$(call GET /v1/customers "$FORGED"); check "payload alterado → 401" 401 "$(status "$r")"

echo
echo "Resultado: $pass ok, $fail fallas (organización $ORG, factura $INV número $NUMBER)"
[ "$fail" -eq 0 ]
