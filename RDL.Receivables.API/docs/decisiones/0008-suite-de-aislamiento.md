# 0008 · Suite de aislamiento y E2E contra dev

- **Fecha:** 2026-09-25
- **Estado:** Aceptada.

## Contexto

El prompt pide los 6 criterios de aislamiento de Platform adaptados, más el bypass del guard (ADR 0001 §4.2), y un E2E
con `cmd/replay`. Platform crea sus propias organizaciones en cada corrida porque es dueña de `core`; Receivables no
puede escribir en `core` ni debe hacerlo.

## Decisión

- `tests/testkit` arma el **router real con los adapters reales** contra dev, con el login `receivables_api`, y se
  niega a correr con un rol que salte RLS. Solo la firma del JWT es simulada (verificador de prueba); la membresía se
  revalida en `core` como en producción.
- Las organizaciones son **dos organizaciones de prueba existentes** de dev, creadas por la suite de Platform, dadas
  por `ISOLATION_ORG_A` e `ISOLATION_ORG_B` (entorno o `.env`). El owner y un miembro suspendido se descubren con la
  sesión de cada organización. Sin esas variables, las suites se saltan con aviso.
- Cada prueba crea sus propias cuentas (por evento) y pagos con ids aleatorios. **No se borra nada** (la app no puede):
  las organizaciones de prueba acumulan datos ficticios.
- `tests/isolation` (`make test-isolation`): (1) endpoints sobre recursos de B con el token de A → 404, token de A con
  org_id de B → 403, B intacto; (2) SELECT directo con la sesión de A no ve filas de B; (3) aplicación cruzada
  rechazada por la base; (4) sin escritura en `core`, `billing`, `fiscal`, `subscriptions`, sin UPDATE/DELETE en
  `audit`, sin outbox ajeno, y el guard no se salta fijando el GUC; (5) sin token, sin org_id o con la membresía
  suspendida no se entra; (6) un `organization_id` en query, header o body no cambia el tenant y un evento de B solo
  afecta a B.
- `tests/e2e` (`make test-e2e`, `scripts/dev/e2e.sh`): factura por evento, pago que la cancela con
  `ReceivableSettled` en el outbox, repetición idempotente, anulación con el saldo de vuelta, reglas del contrato,
  concurrencia, notas, anulación de factura, aging y cobranza.

## Consecuencias

- Si un caso falla por una política que falta en la base, no se debilita el test: se propone la migración a
  `database-platform`.
- La suite depende de que existan esas organizaciones en dev. Si Platform las limpia, hay que elegir otras dos.
