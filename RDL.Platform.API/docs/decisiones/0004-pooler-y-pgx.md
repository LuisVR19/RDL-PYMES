# 0004 · Supavisor en modo transacción y pgx

- **Fecha:** 2026-09-23 · **Estado:** Aceptado

## Contexto

La app se conecta con el login `platform_api` por Supavisor en **modo transacción** (puerto 6543). En ese modo cada transacción puede caer en una conexión física distinta. pgx, por defecto, prepara statements con nombre y los cachea por conexión, lo que produce errores como `prepared statement "stmtcache_…" does not exist` o colisiones entre clientes.

## Decisión

- `DefaultQueryExecMode = pgx.QueryExecModeExec`, con `StatementCacheCapacity = 0` y `DescriptionCacheCapacity = 0`.
  - Se eligió `Exec` y no `SimpleProtocol`: usa el protocolo extendido con statement **sin nombre**, así los parámetros viajan separados del SQL (sin interpolación en el cliente) y no queda estado en la conexión.
- Todo el trabajo con tenant va en una transacción explícita con `set_config(..., true)` (ADR 0003). Nunca `SET` de sesión.
- `statement_timeout` y `application_name` van como parámetros de arranque de la conexión.
- Las **migraciones** usan conexión directa o el modo sesión (5432) con el login `platform_migrate`, porque dependen del `set role` que fija `ALTER ROLE … SET role`.

## Consecuencias

- Hay una ida y vuelta extra por consulta frente a los statements cacheados. Es aceptable para este volumen.
- Si algún día se usa conexión directa sin pooler, se puede volver a `QueryExecModeCacheStatement` cambiando solo `postgres.NewPool`.
