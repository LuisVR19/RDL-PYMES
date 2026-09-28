# ADR 0003 · Analítica sin cookies: Umami

**Estado:** aceptada (2026-09-27, Luis).

## Decisión

Umami, que no usa cookies ni identifica personas: **no hace falta aviso de consentimiento ni página de cookies**. Se
activa con `PUBLIC_UMAMI_SRC` y `PUBLIC_UMAMI_WEBSITE_ID`; vacías, no se carga ningún script. Los clics se
registran con `data-umami-event`: `iniciar-sesion`, `crear-cuenta`, `whatsapp`, `whatsapp-flotante`,
`whatsapp-precios`, `whatsapp-final`, `whatsapp-contadores`, `correo`.

## Pendiente

Dónde se aloja Umami (propio o en la nube). Su origen se agrega a la CSP con `UMAMI_ORIGIN` en el contenedor.

## Consecuencias

Si algún día se cambia por una analítica con cookies (GA4), primero va el aviso de consentimiento y la política de
cookies, y no se carga nada antes del consentimiento (regla del prompt P8c).
