# ADR 0002 · Contacto por WhatsApp en lugar de formulario

**Estado:** aceptada (2026-09-27, Luis).

## Contexto

El prompt P8c pide un formulario de contacto y demo con validación en servidor, antispam (campo trampa, tiempo
mínimo, desafío invisible, límite por IP) y un destino (correo transaccional, función serverless o CRM). Nada de eso
existe todavía, y el hosting depende de P2.

## Decisión

**Sin formulario por ahora.** El llamado final, el de precios y un botón flotante abren WhatsApp (`wa.me`) con un
mensaje prellenado; la página de contadores usa su propio mensaje. El correo queda como enlace `mailto:`. El número y
el correo salen de variables `PUBLIC_*`; sin número, los botones no se muestran.

## Consecuencias

- Sin backend, sin datos personales guardados por el sitio, sin antispam que mantener y sin claves privadas.
- La CSP deja `form-action 'none'`.
- La página `/gracias` y los textos `form.*` del documento 0001 quedan fuera.
- Cuando se retome el formulario: ADR de destino y antispam (Turnstile + límite por IP en la función), sin tocar los
  schemas de las APIs de dominio.
