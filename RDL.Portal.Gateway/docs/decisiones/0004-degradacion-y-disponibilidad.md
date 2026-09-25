# 0004 · Degradación: `availability` aparte del `status` de cada API

**Estado:** propuesta, **necesita confirmación** (ver §3).
**Fecha:** 2026-09-24.

## Contexto

La vista transversal de la arquitectura 2.2 junta tres APIs:

```
Factura FE00100034 · Total ₡113 000 · Hacienda: Aceptada · Saldo ₡63 000
```

Criterio 2: si E-Invoice está caída, el portal tiene que **seguir mostrando la factura**. El prompt P7 sugiere
marcar la parte faltante así:

```json
"fiscal": { "status": "unavailable" }
```

## Problema con esa forma

`status` es el estado del documento electrónico, y su máquina de estados vive en el contrato
(`state-machines/electronic-document.yaml`): `processing, signed, sent, accepted, rejected, contingency, error`.
Meter `unavailable` ahí:

1. **inventa un estado**, que es justo lo que el `CLAUDE.md` de la raíz prohíbe («no se inventan ... eventos ni
   estados»);
2. rompe a cualquier consumidor que haga `switch` sobre el estado —el portal lo hace, en `shared/status`—;
3. confunde dos cosas distintas: *qué dijo Hacienda* y *si pudimos preguntarle*.

## Decisión

Un campo **aparte**, propio del Portal Gateway:

```json
"fiscal": {
  "availability": "available",
  "status": { "electronicDocumentId": "...", "status": "accepted" }
}
"fiscal": { "availability": "unavailable" }
"fiscal": { "availability": "absent" }
```

| `availability` | Significa | Cuándo |
|---|---|---|
| `available` | La API respondió y el dato está | 200 |
| `absent` | La API respondió que no existe. **No es una falla** | 404: un borrador todavía no tiene documento electrónico ni cuenta por cobrar |
| `unavailable` | No se pudo saber | Caída, timeout, 5xx, o la API todavía no está desplegada |

El dato solo viaja cuando es `available`; el constructor de `internal/domain/view` lo garantiza, para que
ningún handler pueda publicar una parte a medias. Un 200 sin cuerpo útil degrada a `unavailable`: para el
portal es lo mismo que no haber contestado.

La fuente **principal** (Billing) no tiene `availability`: si falla, no hay vista. En ese caso se responde el
**Problem Details de Billing tal cual**, con su mismo status y su mismo `type`, no uno inventado por el gateway.

## 3. Lo que hay que confirmar

Esto **se aparta de la forma literal que sugiere el prompt P7**. Se hizo así porque el contrato manda sobre el
prompt (jerarquía de autoridad del `CLAUDE.md` de la raíz). Hace falta:

- [ ] que el equipo lo confirme, y
- [ ] que `openapi/bff-internal.yaml` o un nuevo `openapi/portal-gateway.yaml` del repo de contratos recoja la
      forma final de la vista (PR aparte, 2 aprobaciones + CHANGELOG).

Si se prefiere la forma del prompt, el cambio es de una línea en `internal/adapters/http/overview.go` y sus
DTO; el dominio no cambia.

## Resumen de factura: de dónde sale

`bff-internal.yaml` declara `GET /internal/v1/invoices/{id}/summary` en Billing, pero **Billing no la
implementa** (es un esqueleto del contrato). Para que la vista funcione hoy, el puerto `InvoiceSummaryReader`
tiene dos adapters, elegidos por `BILLING_SUMMARY_SOURCE`:

- `public` (lo de hoy): deriva el resumen de `GET /v1/invoices/{id}`. Trae la factura completa con líneas, así
  que **pesa más de lo necesario**. `customerLegalName` sale de `customerSnapshot.legalName`, que solo existe
  desde la emisión: en borrador queda vacío.
- `internal` (lo definitivo): la ruta del contrato.

**TODO(billing):** implementar la ruta interna y cambiar el valor por defecto a `internal`. Es la tarea que más
desbloquea: de ella dependen también los listados enriquecidos del incremento 5.
