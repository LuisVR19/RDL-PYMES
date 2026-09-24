# Ownership de schemas

Qué API es dueña de cada schema y qué puede hacer cada una en los ajenos (arquitectura 2.1 y 4.5). La fuente
verificable es [`ownership/ownership.yaml`](../ownership/ownership.yaml): `contractsctl validate` rechaza cualquier
escritura sobre el schema de otra API, cualquier `update`/`delete` en `audit` y cualquier servicio o rol que la base
no reconozca. Las APIs pueden usar el YAML en sus tests de permisos.

## Matriz

| Schema | Dueño (escribe y migra) | Lee | Escribe sin ser dueño |
|---|---|---|---|
| `core` | Platform | Billing, fiscal, Receivables (todas las tablas) | — |
| `subscriptions` | Platform | — | — |
| `billing` | Billing | — (fiscal y Receivables reciben los datos por eventos) | — |
| `fiscal` | fiscal (E-Invoice) | Billing: catálogos (CABYS, impuestos, unidades, condiciones de venta, medios de pago, tipos de identificación y de documento, división territorial). Receivables: medios de pago y condiciones de venta | — |
| `receivables` | Receivables | — | — |
| `audit` | database-platform | los 4 servicios | los 4: **solo insert** (append only) |
| `integration` | database-platform | los 4 servicios | los 4: insert, update, delete, limitados por políticas a sus propias filas |
| `shared` | database-platform | tipos y funciones, sin tablas | — |

Roles de base: cada servicio tiene `<servicio>_app` (la aplicación) y `<servicio>_migrator` (sus migraciones).
Los logins los heredan (por ejemplo `platform_api` → `platform_app`).

## Reglas

1. Una API **nunca** actualiza tablas de otra (arquitectura 2.1). Para operar sobre datos ajenos usa eventos.
2. Leer un schema ajeno es **excepcional y explícito**: se declara aquí, tabla por tabla salvo `core`.
3. Cada API migra solo su schema. `audit`, `integration` y `shared` se cambian en `database-platform`.
4. `audit` es append only: ningún rol de aplicación tiene `UPDATE` ni `DELETE`.

## Diferencias con la base de dev (2026-09-24)

| Diferencia | Qué dice el contrato | Qué hay en la base | Acción |
|---|---|---|---|
| Receivables no lee `core` | Lee todas las tablas (necesita revalidar la membresía) | `receivables_app` sin `USAGE` en `core` | Propuesta a `database-platform` antes de construir Receivables |
| Fiscal no lee `billing` | No lee: recibe los datos por eventos | Igual | — |
| Catálogos fiscales vacíos | Los mantiene fiscal | Tablas creadas, sin filas | `TODO(fiscal)` |

## TODOs

- Agregar a `contractsctl` una verificación opcional contra la base (grants reales vs YAML) para el CI de cada API.
