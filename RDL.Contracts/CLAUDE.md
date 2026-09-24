# Contracts (fuente única de verdad entre APIs) — Go

Glosario, máquinas de estado, convenciones, eventos (AsyncAPI 3 + JSON Schema 2020-12) y OpenAPI 3.1 que se prometen
Platform, Billing, fiscal (E-Invoice), Receivables y el BFF. Más el CLI `contractsctl` y el paquete `pkg/events`.
Contexto: `docs/contexto/`. Decisiones: `docs/decisiones/` (leer 0001 antes de tocar cualquier contrato).

# Reglas de plataforma (no modificar sin PR en contracts)
- Cada API escribe solo en su schema (`core`/`subscriptions` Platform, `billing`, `fiscal`, `receivables`),
  más `audit.audit_events` e `integration.*` vía sus políticas.
- Toda tabla de negocio tiene `organization_id uuid NOT NULL`, unicidades y FK compuestas (organization_id, id).
  La organización sale del token verificado, nunca de un body, query o header.
- Montos: decimal. Prohibido float o double (en JSON: string decimal, nunca `number`).
- Fechas en UTC (timestamptz, RFC 3339 con `Z`). Presentación en la zona horaria de la organización.
- Cambios que emiten eventos: entidad + outbox en la MISMA transacción.
- Cada operación sensible genera un audit event con correlationId, en la misma transacción que el cambio.

# Reglas de este repo
- Un schema publicado (`vN` con tag) es inmutable salvo cambios compatibles (agregar un campo opcional).
  Lo incompatible va en `vN+1`, y en Go en un struct `...V2`, sin tocar `V1`.
- No se inventan campos fiscales, códigos de Hacienda, eventos ni estados: `TODO(fiscal)` / `TODO` y a la lista
  de pendientes. No hay especificación de Hacienda en el repo.
- Lo que la base ya impone (dominios de `shared`, `CHECK` de estados, roles de `core.roles`, servicios
  `platform|billing|fiscal|receivables`) manda sobre el documento de arquitectura (ADR 0001).
- El OpenAPI de Platform se importa de `RDL.Platform.API/api/openapi.yaml`. Si no pasa `validate`/`lint`, se reporta
  como hallazgo para Platform; no se retoca para que pase.
- Arquitectura hexagonal: `internal/domain` sin imports de infraestructura, casos de uso en `internal/app`,
  librerías (JSON Schema, YAML) solo en `internal/adapters`. Una regla nueva es un `Check` en `internal/wiring`.
- Una máquina de estado que cambia exige regenerar su diagrama (`go run ./cmd/contractsctl diagram state-machines/<x>.yaml`)
  y pegarlo en `docs/maquinas-de-estado/<x>.md`; validate falla si no coinciden.
- Un cambio incompatible se verifica con `contractsctl breaking -base <dir de la versión publicada>` y se resuelve con una
  versión nueva del evento, nunca modificando la publicada.
- No se ejecutan comandos de git: el repo lo publica el usuario.
- Pendientes y decisiones abiertas: `docs/ESTADO.md`. Agregar ahí todo TODO nuevo.
- Todo cambio de contrato necesita 2 aprobaciones y una entrada en `CHANGELOG.md`.
- Este repo no se conecta a producción ni escribe en ninguna base.

# Antes de terminar cada tarea
```
go build ./... && go vet ./... && golangci-lint run && go test ./... && go run ./cmd/contractsctl validate && go run ./cmd/contractsctl lint
```
Luego revisa tu diff buscando: montos como `number` o `float`, fechas sin zona, campos fiscales inventados, eventos o
estados fuera del documento, contradicciones con Platform y schemas sin `additionalProperties: false`.
