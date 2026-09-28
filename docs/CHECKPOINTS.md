# Checkpoints · RDL PYMES

**Tablero de control del proyecto completo.** Una sola pregunta: qué está hecho, qué falta y quién lo destraba.

**Última revisión:** 2026-09-27 (Billing F5)
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
| P4 | `RDL.Billing.API` `:8081` | ✅ | F2, F3 y **F5** · `-race` · lint · aislamiento 6/6 · integración · E2E 33/33 contra dev (2026-09-27) |
| P5 | `RDL.EInvoice.API` `:8082` | ⬜ | — |
| P6 | `RDL.Receivables.API` `:8083` | ⬜ | — |
| P7 | `RDL.Portal.Gateway` `:8090` | 🟡 | Incrementos 1–5 y 8 · aislamiento 10/10 y E2E 33/33 **contra Platform y Billing reales** (2026-09-27); 6 y 7 bloqueados |
| P8b | `RDL.Web.Portal` `:5173` | 🟡 | Incr. 1–3, 6 y 7 · 184 unit + 43 e2e · **acceso, facturación, inicio y administración probados en vivo** (2026-09-27) |
| P8c | `RDL.Landing` `:4321` | 🟡 | Construida (2026-09-27) · 29 e2e + axe · Lighthouse móvil 99–100 · **datos de prueba y legales en borrador** |

**Dónde está realmente el proyecto:** hay dos APIs de dominio funcionando contra la base de dev y un gateway
que ya habla con Platform de punta a punta (token real, aislamiento entre dos organizaciones, correlación
verificada en el log de Platform), y el portal ya inicia sesión y cambia de organización contra ellos.
Billing ya se probó detrás del gateway (2026-09-27), y las pantallas del portal
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
- [x] Esqueleto de Billing completado en la propuesta, incluido F5 (`cancelInvoice`, referencia y anulación en
      `Invoice`); sigue sin publicar hasta las 2 aprobaciones

### P3 · Platform API — ✅ completo

- [x] Incrementos 1–8; migraciones 00001–00006 aplicadas en dev
- [x] Roles `platform_api` / `platform_migrate`; Custom Access Token Hook activado
- [x] Aislamiento **6/6** · E2E **69/69** con dos usuarios reales
- [ ] Decisiones abiertas: solo un owner gestiona owners · invitaciones sin email · IP de auditoría
- [ ] `operationId` en `/healthz` y `/readyz`; alinear su OpenAPI con el repo de contratos

### P4 · Billing API — ✅ F2 y F3, cerrado en dev (2026-09-27)

- [x] Incrementos 1–9: clientes, productos, borradores, numeración, emisión, `InvoiceIssued` en outbox
- [x] Cálculo exacto con tests de propiedades; `money.Round` ≡ `money.Round5` del contrato
- [x] Migraciones 00001 y 00002 aplicadas
- [x] **Migración 00003** (`branch_fk`) aplicada (2026-09-27)
- [x] **Aislamiento 6/6** e **integración 11/11** contra dev (2026-09-27)
- [x] **E2E 33/33** (2026-09-27). `usuario.e2e2` debe tener activa una organización propia: el script lo verifica
- [ ] ⏳ `make docker` sin verificar (Docker no instalado)
- [x] `GET /internal/v1/invoices/{id}/summary` implementada (2026-09-25), sin líneas; el gateway ya la usa por
      defecto. Pruebas unitarias en verde; aislamiento e integración escritos, sin correr (falta el `.env`)
- [ ] Catálogos `fiscal.*` vacíos: una línea con impuesto responde 422 hasta cargarlos
- [x] **F5** (2026-09-27): notas de crédito y débito, anulación, `CreditNoteIssued`/`DebitNoteIssued`/`InvoiceCancelled`.
      Sin migraciones. Probado en vivo desde el portal; eventos verificados en el outbox de dev
- [ ] Decisiones de F5 para el equipo: misma moneda que la factura, tope de lo acreditado, anular con notas emitidas,
      anular notas, quién genera la nota ante Hacienda al anular (ver `RDL.Billing.API/docs/ESTADO.md`)

### P5 · E-Invoice API — ⬜ no empezado

- [ ] ⛔ Falta la documentación oficial de Hacienda. En `docs/Hacienda/` solo hay la presentación v4.4 y el
      **borrador** de la resolución. Faltan: MH-DGT-RES-0027-2024 oficial, Anexos v4.4, XSD, catálogos en Excel,
      doc del API, política de firma y el CABYS del BCCR
- [ ] Bloquea el módulo D del portal (4 pantallas) y la parte fiscal de la vista transversal

### P6 · Receivables API — ⬜ no empezado

- [ ] Bloquea el módulo E del portal (6 pantallas) y el saldo de la vista transversal
- [ ] `receivables_app` no tiene `USAGE` en `core` pero necesita revalidar membresía → `database-platform`

### P7 · Portal Gateway — 🟡 incrementos 1–5 y 8

- [x] Esqueleto, config validada, OTel, `/healthz`, `/readyz` (crítico vs degradable)
- [x] JWT + propagación de identidad, correlación y trazas
- [x] Tabla de rutas: **56 rutas**, las cuatro APIs declaradas
- [x] Vista transversal `GET /portal/v1/invoices/{id}/overview` con degradación
- [x] `go build` · `go vet` · `golangci-lint` 0 issues · 66 pruebas · humo del binario
- [ ] 🔴 **Confirmar ADR 0004**: usé `availability` aparte del `status`, en vez de la forma del prompt
- [x] Incremento 5 · `GET /portal/v1/invoices` compuesto, 1 llamada por API por página (2026-09-25). Contra las
      rutas por lote **propuestas**; definitivo al aprobarse en contratos
- [ ] ⛔ Incremento 6 · notificaciones → transporte de eventos (P2) sin decidir
- [ ] ⛔ Incremento 7 · read model → no hay schema ni rol para este servicio
- [x] Incremento 8 · `tests/isolation` **8/8** y `scripts/dev/e2e.sh` **31/31** contra Platform real (2026-09-25)
- [x] Los mismos contra **Billing** real (2026-09-27): aislamiento 10/10, E2E 33/33, `overview` de factura emitida
- [x] Arreglo: el problema reenviado de Billing ya no expone su ruta `/internal` en `instance` (2026-09-27)
- [ ] ⏳ Imagen Docker (Docker no instalado)

### P8b · Web Portal — 🟡 incrementos 1–3, 6 y 7

- [x] Armazón, sistema de diseño, 36 pantallas con ruta y permiso, tokens del prototipo
- [x] 76 pruebas unitarias/componentes · 11 e2e con axe (WCAG 2.1 AA, 1440 px y 390 px)
- [ ] ⏳ Incrementos 2–7: acceso, facturación, Hacienda, cobranza, inicio+admin, pulido
- [x] Cableado 1 (2026-09-25): Supabase Auth + adaptador `gateway/`, pantalla 1, cambio de organización seguro,
      ADR 0005 (localStorage + CSP) y 0006. Probado en vivo contra Supabase + gateway + Platform
- [x] Incremento 2 (2026-09-25): pantallas 1–5, 33 y 35 cableadas; invitación real aceptada entre dos usuarios
- [x] Incremento 3 (2026-09-27): pantallas 7–17. Borrador con totales de Billing, emitir, notas y anular (con F5)
- [x] Incremento 6 (2026-09-27): pantallas 6 y 28–32. Inicio suma en el portal (Billing sin resumen ni orden
      descendente); cobranza y Hacienda de Inicio no disponibles hasta P5/P6; la 32 sin API (formulario deshabilitado)
- [x] Incremento 7 · pulido (2026-09-27): login y acceso centrados como el prototipo, formularios centrados
      (decisión de Luis), rejillas de dos columnas, paginación solo si hay otra página, atajos ocultos en móvil,
      carga diferida por módulo (paquete inicial 483 → 316 kB). Pendiente: pruebas visuales
- [ ] ⏳ Incrementos 4 y 5: Hacienda (⛔ P5), cobranza (⛔ P6)
- [ ] Propuestas a contratos: `GET /v1/invoices/summary` + `sort=-issuedAt` (Billing), resumen de saldos (Receivables),
      API de auditoría
- [x] ~~`referencedInvoiceId`/`referenceReason` en `Invoice`~~ (F5). Queda: vencimiento propio de la nota de débito
      (hoy viaja como plazo, `creditTermDays`)
- [x] ~~Chequeo «sin scroll horizontal» de `access.spec.ts`/`shell.spec.ts`~~: los tres specs usan `e2e/helpers.ts` (2026-09-27)
- [ ] Propuestas a contratos/Platform: `GET /v1/invitations/{token}`, `PATCH /v1/me`, separar revocada/usada,
      catálogo de tipos de identificación (hoy del borrador de Hacienda en el portal)
- [ ] Separador de miles: U+202F (README del diseño) vs U+00A0 (prototipo)

### P8c · Landing — 🟡 construida con datos de prueba (2026-09-27)

- [x] Contenido aprobado, principal (12 secciones), `/contadores`, términos y privacidad (borradores, `noindex`), 404
- [x] «Iniciar sesión» con UTM, «Crear cuenta · Próximamente», WhatsApp en lugar de formulario (ADR 0002), Umami (ADR 0003)
- [x] SEO completo, CSP estricta sin `unsafe-inline`, Lighthouse móvil 99–100 en las cuatro categorías
- [ ] ⏳ Datos reales en el `.env` (dominio, contacto, titular) · revisión legal · dónde vive Umami · hosting (P2)
- [ ] ⛔ «Crear cuenta» espera el registro de cuentas en el portal (`/registro`)

---

## 3. Bloqueos, ordenados por lo que destraban

| # | Bloqueo | Destraba | Dueño |
|---|---|---|---|
| 1 | ~~Pasos manuales de dev de Billing~~ ✅ 2026-09-27 | — | — |
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

- [x] ~~`grant usage on schema core to billing_migrator` + `make migrate-up`~~ ✅ 2026-09-27
- [x] ~~Fixtures `0011` + `make test-isolation`~~ ✅ 2026-09-27
- [x] ~~`RDL.Billing.API/.env`~~ ✅ 2026-09-27 (pooler `aws-0-us-east-2`, contraseñas nuevas)
- [x] ~~`.e2e.local` de Billing, Platform y gateway~~ ✅ 2026-09-27 (contraseñas nuevas de los usuarios de prueba)
- [x] ~~`RDL.Platform.API/.env`~~ ✅ 2026-09-27 (contraseñas nuevas de `platform_api`/`platform_migrate`); Platform arranca
- [ ] Crear los tags `v0.1.0` y `v0.2.0` en el repo de contratos al publicarlo

---

## 5. Notas del entorno (revisadas el 2026-09-27)

| Herramienta | Estado | Impacto |
|---|---|---|
| Go 1.27.1 | ✅ Correcto | Compila los 4 módulos Go; resuelve el `replace` a contratos |
| `golangci-lint` 2.14.0 | ✅ **Instalado en esta sesión** | Antes no estaba; es la v2 que piden los `.golangci.yml` |
| `sqlc` v1.31.1 | ✅ Instalado 2026-09-27 en `%USERPROFILE%\go\bin` | Misma versión que el código generado; regenera idéntico |
| `gcc` (WinLibs 16.1) | ✅ Instalado 2026-09-27 | `go test -race` funciona |
| Smart App Control | ✅ Apagado 2026-09-27 | Bloqueaba `go.exe` (no viene firmado, ni con el MSI oficial) |
| `make` | ❌ No instalado | Se corren a mano los comandos de cada `Makefile` |
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

1. ~~Cerrar P4 de verdad~~ ✅ 2026-09-27: aislamiento 6/6, integración 11/11, E2E 33/33.
2. ~~`GET /internal/v1/invoices/{id}/summary` en Billing~~ ✅ hecho el 2026-09-25.
3. ~~Levantar Billing detrás del gateway~~ ✅ 2026-09-27. **Levantar Billing detrás del gateway**: Platform + gateway ya se probaron juntos (2026-09-25). Con Billing
   en `:8081`, `make test-isolation` y `make e2e` del gateway cubren sus casos sin cambios, incluido el `overview`
   contra la ruta interna.
4. ~~Cablear la sesión del portal~~ ✅, ~~incremento 2 (acceso)~~ ✅ y ~~incremento 3 (facturación)~~ ✅ 2026-09-27.
   ~~Incremento 6 del portal (inicio + administración)~~ ✅ 2026-09-27.
   ~~Billing F5~~ ✅ 2026-09-27.
   ~~Incremento 7 del portal (pulido)~~ ✅ 2026-09-27.
   Lo siguiente sin bloqueos: **P6 (Receivables)**, que destraba el incremento 5 del portal, las cifras de cobranza
   de Inicio y el ajuste de saldo de las notas y la anulación.
