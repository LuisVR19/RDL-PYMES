# Checkpoints · RDL PYMES

**Tablero de control del proyecto completo.** Una sola pregunta: qué está hecho, qué falta y quién lo destraba.

**Última revisión:** 2026-09-25
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
| P7 | `RDL.Portal.Gateway` `:8090` | 🟡 | Incrementos 1–5 · aislamiento y E2E 32/32 **contra Platform real**; Billing sin probar |
| P8b | `RDL.Web.Portal` `:5173` | 🟡 | Incr. 1–2 · 127 unit + 27 e2e · **acceso completo probado en vivo** vía gateway |
| P8c | `RDL.Landing` | ⬜ | — |

**Dónde está realmente el proyecto:** hay dos APIs de dominio funcionando contra la base de dev y un gateway
que ya habla con Platform de punta a punta (token real, aislamiento entre dos organizaciones, correlación
verificada en el log de Platform), y el portal ya inicia sesión y cambia de organización contra ellos.
**Falta Billing detrás del gateway** (no arranca en esta máquina sin su `.env`), y las pantallas del portal
siguen siendo marcadores provisionales.

---

## 2. Checkpoints por módulo

### P0 · Contracts — ✅ v0.2.0

- [x] Glosario, convenciones, ownership, 4 máquinas de estado
- [x] AsyncAPI 3 con los 8 eventos · JSON Schema por evento · `InvoiceIssued` v1 completo
- [x] OpenAPI: componentes comunes, Platform importado, esqueletos de Billing/fiscal/Receivables, `bff-internal`
- [x] `contractsctl validate` · `lint` · `breaking` · `pkg/events` sin `float`
- [ ] **Crear los tags `v0.1.0` y `v0.2.0`** ← los publica Luis; hasta entonces las APIs usan `replace`
- [x] `problems/portal-gateway.yaml` y las rutas por lote del BFF, **propuestos sin publicar** (2026-09-25)
- [ ] ⏳ **2 aprobaciones** de esa propuesta → v0.3.0
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
- [x] `GET /internal/v1/invoices/{id}/summary` implementada (2026-09-25), sin líneas; el gateway ya la usa por
      defecto. Pruebas unitarias en verde; aislamiento e integración escritos, sin correr (falta el `.env`)
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
- [x] Incremento 5 · `GET /portal/v1/invoices` compuesto, 1 llamada por API por página (2026-09-25). Contra las
      rutas por lote **propuestas**; definitivo al aprobarse en contratos
- [ ] ⛔ Incremento 6 · notificaciones → transporte de eventos (P2) sin decidir
- [ ] ⛔ Incremento 7 · read model → no hay schema ni rol para este servicio
- [x] Incremento 8 · `tests/isolation` **8/8** y `scripts/dev/e2e.sh` **31/31** contra Platform real (2026-09-25)
- [ ] ⏳ Los mismos contra **Billing** real: sus casos se saltan hasta que Billing arranque en `:8081`
- [ ] ⏳ Imagen Docker (Docker no instalado)

### P8b · Web Portal — 🟡 incremento 1

- [x] Armazón, sistema de diseño, 36 pantallas con ruta y permiso, tokens del prototipo
- [x] 76 pruebas unitarias/componentes · 11 e2e con axe (WCAG 2.1 AA, 1440 px y 390 px)
- [ ] ⏳ Incrementos 2–7: acceso, facturación, Hacienda, cobranza, inicio+admin, pulido
- [x] Cableado 1 (2026-09-25): Supabase Auth + adaptador `gateway/`, pantalla 1, cambio de organización seguro,
      ADR 0005 (localStorage + CSP) y 0006. Probado en vivo contra Supabase + gateway + Platform
- [x] Incremento 2 (2026-09-25): pantallas 1–5, 33 y 35 cableadas; invitación real aceptada entre dos usuarios
- [ ] ⏳ Incrementos 3–7: facturación (el gateway ya sirve Billing), Hacienda, cobranza, inicio+admin, pulido
- [ ] Propuestas a contratos/Platform: `GET /v1/invitations/{token}`, `PATCH /v1/me`, separar revocada/usada,
      catálogo de tipos de identificación (hoy del borrador de Hacienda en el portal)
- [ ] Separador de miles: U+202F (README del diseño) vs U+00A0 (prototipo)

### P8c · Landing — ⬜ no empezado

---

## 3. Bloqueos, ordenados por lo que destraban

| # | Bloqueo | Destraba | Dueño |
|---|---|---|---|
| 1 | Correr los 3 pasos manuales de dev de Billing (§4) | Cierra P4 de verdad | **Luis** |
| 2 | ~~`GET /internal/v1/invoices/{id}/summary` en Billing~~ ✅ 2026-09-25 | — | — |
| 3 | Aprobar las rutas por lote (propuestas sin publicar en contratos) | Listados sin N+1 → datos reales en la pantalla 12 | **Equipo (2 aprobaciones)** |
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
- [ ] **`RDL.Billing.API/.env`** (no existe en esta máquina): `DB_POOLER_HOST` y las contraseñas de
      `billing_api`/`billing_migrate`. Sin él Billing no arranca y el gateway no se puede probar contra ella
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
4. **`BILLING_SUMMARY_SOURCE`** — desde 2026-09-25 vale `internal` por defecto. `public` queda como respaldo
   para una Billing anterior a la ruta interna (trae las líneas: pesa de más).
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
10. **CRLF en los `.go`** — con `core.autocrlf=true` el checkout deja CRLF y `golangci-lint` (gofmt) marca
    todos los archivos. Se normalizó el gateway a LF; un `.gitattributes` con `*.go text eol=lf` lo evita
    en todos los repos.
11. **Idempotencia** — Platform y Billing responden el estado **actual** del recurso al reintentar; las
    convenciones del contrato dicen «la misma respuesta». Hay que decidir cuál vale.

---

## 7. Siguiente paso recomendado

En este orden, porque cada uno destraba al siguiente:

1. **Cerrar P4 de verdad** — los tres pasos manuales de §4. Billing dice «terminado» pero su aislamiento y su
   E2E nunca corrieron; es la brecha más incómoda del proyecto.
2. ~~`GET /internal/v1/invoices/{id}/summary` en Billing~~ ✅ hecho el 2026-09-25.
3. **Levantar Billing detrás del gateway**: Platform + gateway ya se probaron juntos (2026-09-25). Con Billing
   en `:8081`, `make test-isolation` y `make e2e` del gateway cubren sus casos sin cambios, incluido el `overview`
   contra la ruta interna.
4. ~~Cablear la sesión del portal~~ ✅ y ~~incremento 2 (acceso)~~ ✅ 2026-09-25. Lo siguiente del portal es el
   **incremento 3 (facturación)**: las pantallas de Billing se cablean al construirse, porque el gateway ya las sirve.
