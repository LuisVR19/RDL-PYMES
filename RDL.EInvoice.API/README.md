# E-Invoice API (servicio `fiscal`)

Cumplimiento fiscal y comunicación con el Ministerio de Hacienda de Costa Rica para RDL PYMES: comprobantes
electrónicos v4.4 (factura, nota de crédito y nota de débito), firma XAdES-EPES, envío, consulta y contingencia.
Consume los eventos de Billing y publica `ElectronicDocumentAccepted` y `ElectronicDocumentRejected`.

> **Estado:** incrementos 1 a 5 de 13 (esqueleto, logins, JWT y tenancy, consumidor de `InvoiceIssued`, rutas internas del BFF, stub de Hacienda, perfil fiscal, credenciales, certificado, catálogos y CABYS). Ver `docs/ESTADO.md` y
> `docs/PLAN.md`.

## Requisitos

- Go 1.27.1, `golangci-lint` v2. El repo de contratos al lado (`../RDL.Contracts`): `go.mod` lo toma con `replace`.
- Proyecto Supabase de **dev** con los logins `fiscal_api` y `fiscal_migrate` (`scripts/dev/fiscal_login_roles.sql`,
  propuesta a `database-platform`).

## Primer arranque en dev

```sh
cp .env.example .env            # completar DB_POOLER_HOST y las contraseñas de fiscal_api y fiscal_migrate
set -a; . ./.env; set +a
go run ./cmd/migrate mark-baseline   # una sola vez: registra 00001 sin ejecutarla (el schema ya existe)
go run ./cmd/migrate up-by-one       # 00002: columnas y catálogos de la versión 4.4 (expand)
go run ./cmd/migrate status
go run ./cmd/api                     # :8082 → /healthz y /readyz
```

## Comandos

| Comando | Qué hace |
|---|---|
| `make run` | Levanta la API en `:8082` |
| `make run-consumer` | Consumidor de eventos (`:8085`, health). Sin transporte hasta P2: los eventos entran con `make replay` |
| `make replay FILE=…` | Reproduce eventos (uno, un arreglo o un archivo de ejemplos de contratos) por el mismo procesamiento que el consumidor |
| `make import-catalogs` | Carga `catalogs/*.csv` (13 catálogos transcritos del anexo), la ubicación y el CABYS 2025 de `docs/hacienda/catalogos/` con `fiscal_migrate`; registra versión y sha256 en `fiscal.catalog_imports`. `-dry-run` valida sin conectarse |
| `make test` | `go vet` + `go test -race` |
| `make test-integration` | Consumidor, inbox, dead letter y rutas internas contra dev |
| `make test-isolation` | Aislamiento entre tenants contra dev (`ISOLATION_ORG_A/B` en el `.env`) |
| `make lint` | `golangci-lint` con el tag `integration` |
| `make migrate-status` / `migrate-up` / `migrate-up-by-one` | goose sobre `fiscal.goose_db_version` (login `fiscal_migrate`, modo sesión) |
| `make migrate-mark-baseline` | Solo una vez en dev |
| `make docker` | Imagen con el monorepo como contexto |

El worker y las pruebas contra el sandbox de Hacienda llegan con el incremento 9.

## Reproducir eventos

```sh
make replay FILE=../RDL.Contracts/examples/events/invoice-issued.v1.json
```

Una organización sin perfil fiscal no tiene emisor: su `InvoiceIssued` va a dead letter con ese motivo. Reprocesar el
mismo evento no crea un segundo documento (inbox por `eventId` y unicidad por documento comercial).

## Fuentes de Hacienda

`docs/hacienda/` guarda los anexos oficiales v4.4, el borrador de la resolución, la presentación de la DGT y los
**XSD oficiales** (`docs/hacienda/xsd/`, descargados de `https://www.hacienda.go.cr/docs/`). Cada archivo está en
`docs/hacienda/manifest.sha256`; una prueba falla si alguno cambia o aparece sin registrar. Lo que falta (política de
firma, catálogos en Excel, CABYS, documentación del API del sandbox) está en `docs/decisiones/0001-fuentes-de-hacienda.md`.

## Seguridad

- Solo el ambiente de **pruebas** de Hacienda: la API no arranca con `HACIENDA_ENVIRONMENT` distinto de `sandbox`.
- Las credenciales de Hacienda, el `.p12` y su PIN viven en el `SecretStore` (decisión 6), nunca en `.env`, en el
  repo, en logs ni en la base en claro. En desarrollo es `SECRETS_DIR` (por defecto `.secrets/`, ignorado por git):
  archivos AES-256-GCM con la organización como dato asociado, cifrados con `SECRETS_MASTER_KEY` (32 bytes en base64,
  propia de cada máquina). Sin esa llave, configurar credenciales o certificado responde 503.
- `PUT /v1/fiscal-profile` rechaza `environment: production` en V1.
