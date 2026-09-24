# 0003 · Reutilización de `pkg/tenancy` y del esqueleto de Platform

- **Fecha:** 2026-09-24 · **Estado:** Aceptado

## Contexto

El prompt P4 pide reutilizar lo que Platform ya resolvió y probó (tenancy, JWT, sesión RLS, correlación, Problem
Details, health, config) y elegir entre **copiar o importar** `pkg/tenancy`.

## Decisión: copiar, sin cambios

`pkg/tenancy`, `pkg/correlation`, `pkg/requestinfo`, `internal/adapters/auth`, `internal/platform/*` y la sesión RLS
(`internal/adapters/postgres/tx.go`) se **copian** de Platform cambiando solo la ruta del módulo. `pkg/tenancy` es
idéntico línea por línea (se verifica con `diff`).

Por qué no importar:

- El módulo de Platform es `rdl/platform-api`, no está publicado y no tiene versión. Importarlo obligaría a un
  `replace` a una ruta local y ataría el build de Billing al árbol de trabajo de otro repo.
- Importarlo arrastra el `go.mod` completo de Platform, y cualquier cambio en su `pkg/` cambiaría el comportamiento
  de Billing sin un PR aquí.
- La arquitectura ya marca estos paquetes como "candidatos a building-blocks". Cuando exista ese módulo versionado,
  ambos servicios lo importan y se borran las copias. Hasta entonces, copiar es más barato que acoplar.

Riesgo aceptado: las copias pueden divergir. Mitigación: los tests de `pkg/tenancy` también se copiaron, y un
cambio de seguridad en tenancy de Platform debe repetirse aquí (anotarlo en el PR de Platform).

## Adaptaciones propias de Billing

| Qué | Platform | Billing |
|---|---|---|
| Rutas protegidas | `/v1/me` solo con JWT; `/v1/organizations/current/...` con TenantContext | **todo `/v1/...` exige TenantContext**: Billing no tiene operaciones sin organización (convenciones D7) |
| `TxManager` | `WithinUserTx` y `WithinTenantTx` | solo `WithinTenantTx` |
| Revalidación de membresía | lee `core` como dueño del schema | lee `core` con los GRANT de la propuesta 0002 (misma consulta) |
| Matriz de permisos | `internal/domain/membership` | `internal/domain/permission` (propuesta del prompt: owner, admin y biller escriben; el resto lee) |
| Problem types | `urn:rdl:platform:problem:*` | `urn:rdl:billing:problem:*` (`problems/billing.yaml` del contrato) |
| Puerto local | `:8080` | `:8081` (el `servers` del OpenAPI de Billing en el contrato) |

## Dependencias

Las mismas que Platform (su ADR 0005) en las mismas versiones: `pgx/v5`, `goose/v3`, `google/uuid`, OpenTelemetry,
`golang-jwt/jwt/v5` + `MicahParks/keyfunc/v3`. Las que pide el prompt para incrementos siguientes (librería decimal,
`santhosh-tekuri/jsonschema/v6`, `pgregory.net/rapid`, el módulo de contratos, `go-playground/validator`) se agregan
al necesitarse y se registran aquí.
