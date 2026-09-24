# 0005 · Router HTTP y dependencias

- **Fecha:** 2026-09-23 · **Estado:** Aceptado

## Decisión

- **Router:** `net/http.ServeMux` de la biblioteca estándar (patrones `MÉTODO /ruta/{param}`, Go 1.22+). No se usa `chi`: los patrones nativos cubren las rutas de F1 y F3, y el orden de los middlewares se arma por composición explícita en `adapters/http/router.go`.
- **Toolchain:** Go 1.27.1, la versión instalada en la máquina de desarrollo.

## Dependencias directas y por qué

| Dependencia | Motivo |
|---|---|
| `jackc/pgx/v5` | Driver y pool de PostgreSQL (lo pide el prompt) |
| `pressly/goose/v3` | Migraciones, embebidas en `cmd/migrate` (lo pide el prompt) |
| `google/uuid` | Parseo y validación de UUID (correlation id, ids) y UUIDv5 para la idempotencia del alta de organización (0001 §5.3). La biblioteca estándar no trae UUID |
| `go.opentelemetry.io/otel` (+ sdk, exportadores OTLP/HTTP, `otelhttp`) | Trazas y métricas (arquitectura 8.2) |
| `golang-jwt/jwt/v5` + `MicahParks/keyfunc/v3` | Verificación de JWT de Supabase con JWKS en caché y refresco (el prompt admite esta opción). Solo algoritmos asimétricos: ES256, RS256, EdDSA |
| `sqlc` (herramienta, no dependencia de runtime) | Consultas tipadas en `queries/`. `sqlc/external.sql` describe los objetos de database-platform solo para el tipado |

Las dependencias de los incrementos siguientes (`go-playground/validator`, `testcontainers-go`) se agregan al necesitarse y se registran aquí.
