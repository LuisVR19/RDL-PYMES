# 0002 · Tabla de rutas, lista blanca de headers y APIs que todavía no existen

**Estado:** aceptada (incremento 1–4).
**Fecha:** 2026-09-24.

## Contexto

El Portal Gateway es el único punto de entrada del portal. La tentación evidente es un reverse proxy que
reenvíe `/portal/v1/*` a la API que corresponda según un prefijo. Eso expondría, sin querer, cualquier ruta
que una API agregue después —incluidas las **internas**, que no deben ser accesibles desde el navegador— y
cualquier header que el cliente mande.

Además, dos de las cuatro APIs (E-Invoice y Receivables) no existen todavía.

## Decisión

**1. Una sola tabla declarativa** en `internal/domain/routes`. Cada fila declara método, ruta pública, tipo
(paso directo o composición), API destino y ruta destino. El router recorre la tabla: **lo que no está
declarado es 404** y nunca llega a ninguna API. Agregar una pantalla es agregar filas.

**2. Listas blancas de headers**, también en el dominio:

- *Hacia las APIs*: `Authorization`, `X-Correlation-Id`, `Idempotency-Key`, `Accept-Language`, `Accept`,
  `Content-Type`, `traceparent`, `tracestate`. Todo lo demás se descarta (`Cookie`, `X-Forwarded-For`,
  cualquier `X-Organization-Id` que alguien intente).
- *Hacia el navegador*: `Content-Type`, `Content-Language`, `X-Correlation-Id`, `Location`, `Retry-After`,
  `ETag`. Nada de cabeceras internas de las APIs.

`Authorization` se borra de lo copiado y lo vuelve a poner el cliente de salida con el token **verificado**
del contexto: así es imposible reenviar un encabezado que no pasó por la verificación.

**3. Se borran de la query** `organizationId`, `organization_id`, `orgId` y `org_id`. El gateway no agrega una
organización y tampoco deja que el cliente lo intente: sale del token y la resuelve cada API contra su base.

**4. Las cuatro APIs se declaran desde el día uno.** Las rutas de E-Invoice y Receivables existen aunque sus
repos no. Sin URL configurada responden **503 `upstream-not-configured`**, que es distinto de 404: le dice al
portal «esta función todavía no está», no «esta ruta no existe».

**5. `/readyz` distingue crítico de degradable.** El JWKS es crítico (sin él no se autentica a nadie) → 503.
Cada API destino es degradable → 200 con `status: "degraded"`. Una API que no está configurada no se chequea.

## Por qué 4 y 5

Si las rutas de fiscal y receivables aparecieran «cuando toque», habría que reabrir el router, la
configuración, los health checks y las pruebas, justo cuando el equipo esté ocupado con la integración real
con Hacienda. Declararlas ahora cuesta una tabla y hace que la degradación se pruebe **de verdad** hoy, no
simulando una caída.

Y si `/readyz` exigiera las cuatro APIs, con E-Invoice sin desplegar el gateway nunca estaría listo: el
balanceador lo sacaría y el portal se quedaría sin **nada**, cuando facturar funciona perfectamente.

## Consecuencias

- Agregar una pantalla = agregar filas + su prueba. No se escribe otro proxy.
- El router valida al arrancar: una composición declarada sin caso de uso **impide arrancar** en lugar de
  servir una ruta a medias.
- Un header nuevo que una API necesite hay que agregarlo a la lista blanca a propósito.
- El método no va en el patrón del mux: `GET /portal/v1/catalogs/{catalog}` y el literal
  `/portal/v1/catalogs/cabys` se registran como ambiguos si el método está presente. Sin método manda la
  precedencia normal (lo literal gana al comodín) y cada ruta despacha por método, con su 405 propio.
- `HEAD` no está declarado en ninguna ruta: responde 405. El portal no lo usa.
- **La autenticación va antes que el enrutamiento.** Una petición sin token válido recibe 401 aunque la ruta
  no exista: a quien no se identificó no se le dice qué rutas hay. Con token válido, una ruta no declarada sí
  responde 404.
