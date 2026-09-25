# 0006 · Cliente HTTP del Portal Gateway escrito a mano

- Estado: aceptada · 2026-09-25 · desviación del stack del prompt P8b

## Contexto
P8b pide `openapi-fetch` con tipos generados por `openapi-typescript` desde el OpenAPI del Portal Gateway. Hoy ese
OpenAPI describe las rutas de **paso directo** con una respuesta genérica (`Upstream`): el gateway reenvía lo que
responde la API dueña sin declararlo. Los tipos generados serían `unknown` para casi todo lo que usa el portal.

## Decisión
- `src/shared/api/gateway/http.ts` es el **único `fetch`** del portal: pone `Authorization` (token del usuario,
  pedido a Supabase en cada llamada), un `X-Correlation-Id` nuevo por llamada y la `Idempotency-Key` que le pase
  quien llama; convierte Problem Details en `ApiError` con el código de referencia del gateway. Nunca agrega una
  organización.
- `src/shared/api/gateway/index.ts` implementa los puertos. Sus DTO se escriben a mano, con solo los campos que el
  portal usa, siguiendo el OpenAPI de la API dueña del repo de contratos (hoy `openapi/platform.yaml`).
- Las pantallas siguen sin conocer HTTP: usan los puertos (ADR 0003).

## Cuándo revisarlo
Cuando el OpenAPI del gateway declare los cuerpos (o importe los de contratos), se generan los tipos y se cambia
este cliente por `openapi-fetch`; los puertos no cambian.
