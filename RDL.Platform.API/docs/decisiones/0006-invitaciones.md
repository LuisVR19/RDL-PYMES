# 0006 · Invitaciones

- **Fecha:** 2026-09-23 · **Estado:** Aceptado (revisar en equipo los puntos marcados con ⚠)

## Decisión

1. **Token:** 32 bytes aleatorios (`crypto/rand`) en base64url con prefijo `inv_`. En `core.invitations.token_hash` solo se guarda el SHA-256 en hex minúscula (dominio `shared.sha256_hex`). El token nunca se escribe en la auditoría, los logs ni la tabla de idempotencia.
2. **Vigencia:** 7 días. Una invitación pendiente vencida se trata como `expired` al leerla. Al volver a invitar al mismo email se marca `expired` para liberar el índice único de pendientes.
3. ⚠ **Idempotencia del alta:** el token solo se devuelve en la primera respuesta. Un reintento con la misma `Idempotency-Key` devuelve 201 con la misma invitación, pero **sin token**, porque no se puede reconstruir a partir del hash. Es una desviación consciente de "misma clave → misma respuesta". La alternativa (derivar el token con HMAC de un secreto del servidor) agrega un secreto que habría que gestionar y rotar.
4. **Aceptación** (`POST /v1/invitations/{token}/accept`):
   - con la sesión del invitado, la política `invitations_invitee` solo deja ver invitaciones dirigidas a **su email**; un token de otra persona responde 404;
   - después, dentro de la organización de la invitación, se revalida todo bajo `FOR UPDATE` (pendiente, no vencida, mismo email) y en **una transacción** se crea la membresía (o se reactiva una suspendida), se marca la invitación como aceptada, se fija la organización activa si el usuario no tenía y se audita;
   - la autoridad para escribir en esa organización sale del token de un solo uso más el email verificado del JWT, nunca de un id enviado por el cliente.
5. ⚠ **Solo un owner puede invitar con rol owner.** Es la misma regla que en la gestión de miembros (ADR 0003 y `membership.PlanChange`).
6. **Revocar** es idempotente: revocar una invitación ya revocada o vencida devuelve 204. Revocar una aceptada responde 409.

## Consecuencias y pendientes

- ⚠ **El token viaja en la ruta**, como define el prompt. Los logs propios registran el patrón (`/v1/invitations/{token}/accept`) y no la URL, pero un proxy o balanceador podría registrar la URL completa. El riesgo es acotado, porque el token es de un solo uso y solo sirve al titular del email. Si el contrato lo permite, conviene moverlo al cuerpo.
- TODO(notificaciones): enviar el enlace por email. Hoy la API devuelve `token` y `acceptPath` al crear la invitación para que el admin los comparta.
- TODO(contracts): los eventos de invitación y membresía no están en el catálogo 6.2, así que no se publican en el outbox.
