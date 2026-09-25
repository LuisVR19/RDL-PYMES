# Portal Gateway

BFF (*Backend For Frontend*) del portal web del SaaS de facturación electrónica para PYMES de Costa Rica.
Es el **único punto de entrada** de `RDL.Web.Portal`: verifica al usuario, reenvía sus peticiones a la API
dueña de cada dato y **compone** lo que una pantalla necesita de varias.

El ejemplo de la arquitectura (2.2):

```
Factura FE00100034
Total: ₡113 000  |  Hacienda: Aceptada  |  Saldo: ₡63 000
```

El total es de Billing, el estado fiscal de E-Invoice y el saldo de Receivables. El gateway los junta **sin
crear dependencias entre dominios**.

Go 1.27, `net/http` estándar, arquitectura hexagonal. **Sin base de datos y sin reglas de negocio.**
Alcance: incrementos 1 a 4 del planning P7. Estado y pendientes: [`docs/ESTADO.md`](docs/ESTADO.md).

> **Hoy solo existen dos de las cuatro APIs.** Platform y Billing están desplegables; E-Invoice y Receivables
> todavía no. Sus rutas **están declaradas igual**: responden 503 `upstream-not-configured` y sus partes de
> una vista salen marcadas como `unavailable`. Ver [ADR 0002](docs/decisiones/0002-tabla-de-rutas-y-headers.md).

## Qué garantiza

- **El token es del usuario.** Se verifica contra el JWKS de Supabase y se reenvía byte por byte. Nunca un
  token de servicio. Un token inválido se corta aquí y **ninguna API se entera**.
- **La organización no la decide el gateway.** Sale del `org_id` del token y la revalida cada API contra su
  base. El gateway no la agrega a ninguna llamada y la borra si el cliente la manda por query.
- **Solo existe lo declarado.** Las 55 rutas viven en una tabla (`internal/domain/routes`); lo demás es 404.
  Las rutas internas de las APIs nunca se exponen al navegador.
- **Una API caída no tumba la pantalla.** La vista transversal sale con la parte faltante marcada
  (criterio 2), y `/readyz` distingue una dependencia crítica de una degradable.
- **Los comandos no se reintentan.** Un `POST .../issue` reintentado podría emitir dos facturas; reintenta el
  cliente con su `Idempotency-Key`.
- **Los errores son los de la API dueña**, con su mismo status y su mismo `type`.

## Rutas

Todo cuelga de `/portal/v1/...` y exige `Authorization: Bearer <access token de Supabase>`.
Contrato completo en [`api/openapi.yaml`](api/openapi.yaml); la lista viva sale del propio binario:

```sh
go run ./cmd/gateway -routes
```

| Grupo | API dueña | Pantallas |
|---|---|---|
| `/me`, `/me/memberships`, `/me/active-organization` | Platform | 2, 5, 33 |
| `/organizations/current[/users,/invitations,/branches]`, `/invitations/{token}/accept` | Platform | 3, 4, 28–31 |
| `/customers`, `/products`, `/invoices`, `/document-sequences` | Billing | 7–15, 28 |
| `/fiscal-profile`, `/establishments`, `/electronic-documents`, `/catalogs` | E-Invoice | 18–21 |
| `/receivables`, `/payments`, `/payment-applications` | Receivables | 22–27 |
| `/invoices/{id}/overview` | **composición** | 15 |
| `/healthz`, `/readyz` | — | público |

## Levantar en local

Requisitos: Go 1.27.1 y `golangci-lint`. No hace falta base de datos ni `sqlc`.

1. Copie `.env.example` a `.env`. Con `SUPABASE_URL`, `PLATFORM_API_URL` y `BILLING_API_URL` basta;
   deje vacías `FISCAL_API_URL` y `RECEIVABLES_API_URL` hasta que esas APIs existan.
2. Levante Platform (`:8080`) y Billing (`:8081`) en sus repos.
3. `make run` → `:8090`. `curl localhost:8090/readyz`.

Con E-Invoice y Receivables sin configurar, `/readyz` responde **200 `ok`** (no se chequea lo que no está
configurado) y las pantallas de Hacienda y cobranza reciben 503 `upstream-not-configured`.

```sh
TOKEN=...   # access token de Supabase con org_id
curl -H "Authorization: Bearer $TOKEN" localhost:8090/portal/v1/customers
curl -H "Authorization: Bearer $TOKEN" localhost:8090/portal/v1/invoices/<id>/overview
```

El `overview` responderá con el total real de Billing y `"availability":"unavailable"` en `fiscal` y
`receivable`. Eso es lo correcto hoy, no un error.

## Comandos

| Comando | Qué hace |
|---|---|
| `make run` | Levanta el gateway |
| `make build` | Compila y deja `bin/gateway` |
| `make test` | `go vet` + `go test -race` |
| `make lint` | golangci-lint |
| `make routes` | Imprime la tabla de rutas |
| `make docker` | Imagen distroless, usuario no root |

Antes de dar algo por terminado:

```sh
go build ./... && go vet ./... && golangci-lint run && go test ./...
```

## Cómo agregar una pantalla

1. Agregue su fila (o filas) a `internal/domain/routes/routes.go`, con el `Why` que la justifica.
2. Agréguela a `api/openapi.yaml`. `TestOpenAPIMatchesRouteTable` falla si se olvida de uno de los dos.
3. Si es una composición: el caso de uso en `internal/app`, su modelo en `internal/domain/view` con la regla
   de degradación, y conéctelo en `composedHandler` (`internal/adapters/http/router.go`). Una composición
   declarada sin caso de uso **impide arrancar**, a propósito.
4. Pruebas: propagación, tenancy y —si compone— degradación de cada parte.

## Estructura

```
cmd/gateway/             composition root: configuración, wiring y arranque
internal/
  domain/
    routes/              LA tabla de rutas y las listas blancas de headers. Datos puros
    view/                modelos de vista y reglas de degradación (availability)
  app/                   casos de uso + puertos por API (ports.go). Se prueban con fakes
  adapters/
    auth/                verificación del JWT contra el JWKS de Supabase
    downstream/          clientes de las cuatro APIs: token, timeouts, reintentos, clasificación de errores
    http/                router guiado por la tabla, proxy, composiciones, CORS, Problem Details
  wiring/                arma handlers y clientes; lo usan cmd/ y las pruebas
  platform/              config, logger, telemetría, health
pkg/
  correlation/           X-Correlation-Id (reutilizado de Platform)
  identity/              el usuario verificado y su token, en el contexto
api/openapi.yaml         rutas públicas
docs/                    ESTADO, decisiones (ADR), propuestas para el repo de contratos, contexto
```

## Pendientes

Lo que **no** trae esta versión, con su motivo (detalle en [`docs/ESTADO.md`](docs/ESTADO.md)):

| Pendiente | Bloqueado por |
|---|---|
| Listados enriquecidos sin N+1 (pantalla 12) | Rutas por lote, propuestas al repo de contratos |
| Vista transversal contra la ruta interna de Billing | Billing no implementa `/internal/v1/invoices/{id}/summary` |
| Notificaciones en tiempo real (pantalla 34) | Transporte de eventos sin decidir (P2) |
| Read model | Decisión de almacenamiento (no hay schema ni rol para este servicio) |
| Pantalla 32 · Exportar auditoría | Ninguna API expone `audit.audit_events` |
