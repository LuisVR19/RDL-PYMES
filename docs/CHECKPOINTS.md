# Checkpoints · RDL PYMES

**Tablero de control del proyecto completo.** Una sola pregunta: qué está hecho, qué falta y quién lo destraba.

**Última revisión:** 2026-09-24
**Cómo se usa:** este archivo es el índice de estado. El detalle de cada módulo vive en su
`<módulo>/docs/ESTADO.md`, que manda sobre este resumen. Al cerrar un incremento se actualizan los dos.

Leyenda: ✅ terminado y verificado · 🟡 hecho pero **sin verificar de punta a punta** · ⏳ pendiente ·
⛔ bloqueado por otro · ⬜ no empezado

---

## 1. Panorama

| Fase | Módulo | Estado | Verificado contra |
|---|---|---|---|
| P0 | `RDL.Contracts` v0.2.0 | ✅ | `contractsctl validate` + `lint` + tests |
| P3 | `RDL.Platform.API` `:8080` | ✅ | Aislamiento 6/6 · E2E 69/69 contra dev |
| P4 | `RDL.Billing.API` `:8081` | 🟡 | Tests en verde; **aislamiento y E2E escritos pero sin correr** |
| P5 | `RDL.EInvoice.API` `:8082` | ⬜ | — |
| P6 | `RDL.Receivables.API` `:8083` | ⬜ | — |
| P7 | `RDL.Portal.Gateway` `:8090` | 🟡 | 66 pruebas con APIs falsas; **nunca contra APIs reales** |
| P8b | `RDL.Web.Portal` `:5173` | 🟡 | 76 unit + 11 e2e; **solo datos simulados, sin cableado** |
| P8c | `RDL.Landing` | ⬜ | — |

**Dónde está realmente el proyecto:** hay dos APIs de dominio funcionando contra la base de dev, un gateway
recién armado que nunca las ha tocado, y un portal que todavía no llama a nadie. **Ninguna pieza se ha
probado conectada con otra.** Ese es el hueco grande.

---

## 2. Checkpoints por módulo

### P0 · Contracts — ✅ v0.2.0

- [x] Glosario, convenciones, ownership, 4 máquinas de estado
- [x] AsyncAPI 3 con los 8 eventos · JSON Schema por evento · `InvoiceIssued` v1 completo
- [x] OpenAPI: componentes comunes, Platform importado, esqueletos de Billing/fiscal/Receivables, `bff-internal`
- [x] `contractsctl validate` · `lint` · `breaking` · `pkg/events` sin `float`
- [ ] **Crear los tags `v0.1.0` y `v0.2.0`** ← los publica Luis; hasta entonces las APIs usan `replace`
- [ ] Recibir `problems/portal-gateway.yaml` (propuesto en `RDL.Portal.Gateway/docs/propuestas/`)
- [ ] Recibir las rutas por lote para los listados del BFF
- [ ] Completar el esqueleto de Billing (`ProductPatch`, campos de `Invoice`, filtros, `used`)

### P3 · Platform API — ✅ completo

- [x] Incrementos 1–8; migraciones 00001–00006 aplicadas en dev
- [x] Roles `platform_api` / `platform_migrate`; Custom Access Token Hook activado
- [x] Aislamiento **6/6** · E2E **69/69** con dos usuarios reales
- [ ] Decisiones abiertas: solo un owner gestiona owners · invitaciones sin email · IP de auditoría
- [ ] `operationId` en `/healthz` y `/readyz`; alinear su OpenAPI con el repo de contratos

### P4 · Billing API — 🟡 F2 y F3 implementadas, endurecimiento a medias

- [x] Incrementos 1–9: clientes, productos, borradores, numeración, emisión, `InvoiceIssued` en outbox
- [x] Cálculo exacto con tests de propiedades; `money.Round` ≡ `money.Round5` del contrato
- [x] Migraciones 00001 y 00002 aplicadas
- [ ] ⏳ **Migración 00003** (`branch_fk`): falla por permisos. Falta `grant usage on schema core to billing_migrator`
- [ ] ⏳ **Aislamiento nunca corrido**: faltan los fixtures `scripts/dev/0011_billing_isolation_fixtures.sql`
- [ ] ⏳ **E2E nunca corrido**: falta `.e2e.local` con usuario de prueba
- [ ] ⏳ `make docker` sin verificar (Docker no instalado)
- [ ] ⛔ **Implementar `GET /internal/v1/invoices/{id}/summary`** ← lo necesita el gateway
- [ ] Catálogos `fiscal.*` vacíos: una línea con impuesto responde 422 hasta cargarlos

### P5 · E-Invoice API — ⬜ no empezado

- [ ] ⛔ Falta la documentación oficial de Hacienda. En `docs/Hacienda/` solo hay la presentación v4.4 y el
      **borrador** de la resolución. Faltan: MH-DGT-RES-0027-2024 oficial, Anexos v4.4, XSD, catálogos en Excel,
      doc del API, política de firma y el CABYS del BCCR
- [ ] Bloquea el módulo D del portal (4 pantallas) y la parte fiscal de la vista transversal

### P6 · Receivables API — ⬜ no empezado

- [ ] Bloquea el módulo E del portal (6 pantallas) y el saldo de la vista transversal
- [ ] `receivables_app` no tiene `USAGE` en `core` pero necesita revalidar membresía → `database-platform`

### P7 · Portal Gateway — 🟡 incrementos 1–4

- [x] Esqueleto, config validada, OTel, `/healthz`, `/readyz` (crítico vs degradable)
- [x] JWT + propagación de identidad, correlación y trazas
- [x] Tabla de rutas: **55 rutas**, las cuatro APIs declaradas
- [x] Vista transversal `GET /portal/v1/invoices/{id}/overview` con degradación
- [x] `go build` · `go vet` · `golangci-lint` 0 issues · 66 pruebas · humo del binario
- [ ] 🔴 **Confirmar ADR 0004**: usé `availability` aparte del `status`, en vez de la forma del prompt
- [ ] ⛔ Incremento 5 · listados sin N+1 → rutas por lote en contratos
- [ ] ⛔ Incremento 6 · notificaciones → transporte de eventos (P2) sin decidir
- [ ] ⛔ Incremento 7 · read model → no hay schema ni rol para este servicio
- [ ] ⏳ Incremento 8 · `tests/isolation` (carpeta vacía), e2e, imagen Docker
- [ ] ⏳ **Nunca corrió contra Platform y Billing reales**

### P8b · Web Portal — 🟡 incremento 1

- [x] Armazón, sistema de diseño, 36 pantallas con ruta y permiso, tokens del prototipo
- [x] 76 pruebas unitarias/componentes · 11 e2e con axe (WCAG 2.1 AA, 1440 px y 390 px)
- [ ] ⏳ Incrementos 2–7: acceso, facturación, Hacienda, cobranza, inicio+admin, pulido
- [ ] ⏳ **Cableado**: adaptador `gateway/` contra `/portal/v1` — ya tiene a quién llamar
- [ ] Separador de miles: U+202F (README del diseño) vs U+00A0 (prototipo)

### P8c · Landing — ⬜ no empezado

---

## 3. Bloqueos, ordenados por lo que destraban

| # | Bloqueo | Destraba | Dueño |
|---|---|---|---|
| 1 | Correr los 3 pasos manuales de dev de Billing (§4) | Cierra P4 de verdad | **Luis** |
| 2 | `GET /internal/v1/invoices/{id}/summary` en Billing | Vista transversal definitiva + incremento 5 del gateway | **Billing** |
| 3 | Rutas por lote en contratos | Listados sin N+1 → datos reales en la pantalla 12 | **Contracts (PR)** |
| 4 | Adaptador `gateway/` en el portal | Quita los datos simulados de 24 pantallas | **Web Portal** |
| 5 | Documentación oficial de Hacienda | Todo P5, y con él el módulo D | **Externo** |
| 6 | Transporte de eventos | Notificaciones en tiempo real, workers | **P2** |
| 7 | Almacenamiento del read model | Incremento 7 del gateway | **Luis** |
| 8 | Tags de contratos | Quitar los `replace` de los `go.mod` y el Dockerfile especial de Billing | **Luis** |

---

## 4. Acciones manuales pendientes en dev

Proyecto Supabase dev `dzlsnsstuqpxvwegeqcy`. Nada de esto lo puede hacer un agente.

- [ ] **SQL Editor como `postgres`:** `grant usage on schema core to billing_migrator;` → luego
      `cd RDL.Billing.API && make migrate-up` (aplica `00003_branch_fk`)
- [ ] **SQL Editor:** correr `RDL.Billing.API/scripts/dev/0011_billing_isolation_fixtures.sql` → luego
      `make test-isolation`
- [ ] **`RDL.Billing.API/.e2e.local`** con `E2E_EMAIL`/`E2E_PASSWORD` (los usuarios ya existen del E2E de
      Platform) → `make run` y `bash scripts/dev/e2e.sh`
- [ ] Crear los tags `v0.1.0` y `v0.2.0` en el repo de contratos al publicarlo

---

## 5. Notas del entorno (revisadas el 2026-09-24)

| Herramienta | Estado | Impacto |
|---|---|---|
| Go 1.27.1 | ✅ Correcto | Compila los 4 módulos Go; resuelve el `replace` a contratos |
| `golangci-lint` 2.14.0 | ✅ **Instalado en esta sesión** | Antes no estaba; es la v2 que piden los `.golangci.yml` |
| `sqlc` | ❌ No instalado | `make sqlc` de Platform y Billing no corre. El gateway no lo usa |
| Detector de carreras | ❌ **No funciona** | Necesita cgo y no hay `gcc`. **`make test` de Platform y Billing falla**, porque usan `go test -race` |
| Docker | ❌ No instalado | `make docker` sin verificar en los tres servicios |
| `%USERPROFILE%\go\bin` | ⚠️ Fuera del `PATH` | `golangci-lint` hay que llamarlo por ruta completa en Git Bash |

**Para habilitar `-race`:** instalar un toolchain de C (MSYS2 o WinLibs) y dejar `gcc` en el `PATH`.
Mientras tanto, `go test` sin `-race` pasa en todos los módulos.

---

## 6. Detalles sueltos que conviene no perder

1. **`RDL.Platform.API/f:86`** — archivo vacío de 0 bytes, no rastreado por git. Resto de una redirección mal
   tipeada. Inofensivo; se puede borrar.
2. **`CLAUDE.md` de Billing** — su cadena de cierre no incluye `make test-integration`, que sí existe en su
   `Makefile` y es la única que ejerce el SQL real de los adapters.
3. **Desviación del gateway (ADR 0004)** — el prompt P7 sugiere `fiscal: { status: "unavailable" }`; se
   implementó `availability` aparte para no inventar un estado en la máquina del documento electrónico.
   **Necesita el visto bueno del equipo**; revertirlo es una línea.
4. **`BILLING_SUMMARY_SOURCE=public` es temporal** — el gateway deriva el resumen del detalle público de la
   factura porque Billing no expone la ruta interna. Trae la factura completa con líneas: **pesa de más**.
5. **`customerLegalName` vacío en borradores** — el snapshot del cliente solo existe desde la emisión. No es
   un bug del gateway; la pantalla tiene que contemplarlo.
6. **La autenticación va antes que el enrutamiento en el gateway** — sin token válido todo responde 401,
   aunque la ruta no exista. Es deliberado (no se le dice a un desconocido qué rutas hay), pero conviene
   saberlo al depurar.
7. **La fecha de `RDL.Billing.API/docs/ESTADO.md` dice 2026-09-25**, un día por delante del resto. Revisar
   cuál es la buena.
8. **Los catálogos `fiscal.*` están vacíos** — hoy una línea de factura con impuesto responde 422. No es un
   bug: es que no hay catálogos oficiales de Hacienda todavía.
9. **Consolidar `pkg/tenancy`, `pkg/correlation` y compañía** en un módulo *building-blocks* versionado: hoy
   están copiados en Platform, Billing y el gateway, y ya empezaron a divergir (el gateway usa
   `pkg/identity`, sin TenantContext, porque no tiene base).
10. **Idempotencia** — Platform y Billing responden el estado **actual** del recurso al reintentar; las
    convenciones del contrato dicen «la misma respuesta». Hay que decidir cuál vale.

---

## 7. Siguiente paso recomendado

En este orden, porque cada uno destraba al siguiente:

1. **Cerrar P4 de verdad** — los tres pasos manuales de §4. Billing dice «terminado» pero su aislamiento y su
   E2E nunca corrieron; es la brecha más incómoda del proyecto.
2. **`GET /internal/v1/invoices/{id}/summary` en Billing** — deja la vista transversal en su forma definitiva.
3. **Levantar Platform + Billing + Gateway juntos** y probar el paso directo y el `overview` con un token
   real. Es la primera vez que tres piezas se hablarían.
4. **Cablear el portal** contra `/portal/v1` — con eso 24 de las 36 pantallas dejan de ser simuladas.
