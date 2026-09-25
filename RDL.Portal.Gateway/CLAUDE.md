# Portal Gateway (BFF del portal web) — Go

Único punto de entrada de `RDL.Web.Portal`. Recibe al usuario, reenvía sus peticiones a las APIs dueñas
(Platform, Billing, E-Invoice/`fiscal` y Receivables) y **compone** lo que una pantalla necesita de varias.
Es un mesero: pide, junta y entrega; no cocina.
Contexto: `docs/contexto/` (arquitectura 2.1, 2.2, 5.2, 8.2 y criterios 1, 2 y 9; planning P7).
Decisiones: `docs/decisiones/` (leer 0001 y 0002 antes de tocar rutas o propagación).
Contratos: `../RDL.Contracts`. **El contrato manda**: si este archivo o un prompt lo contradicen, detente y pregunta.

# Reglas de plataforma (no modificar sin PR en contracts)
- Este servicio **no tiene base de datos**. No escribe en ningún schema, no se conecta a Postgres y no tiene
  rol de base. Lo que parezca necesitar una tabla es una propuesta a `database-platform`, no una migración aquí.
- La organización sale SOLO del `org_id` del JWT verificado, y **la revalida cada API** contra su base: el
  gateway no tiene acceso a `core` y no revalida membresías.
- Montos: string decimal, tal como los dio su API dueña. **Prohibido `float`**, y prohibido recalcular: si hay
  que sumar, redondear o convertir moneda, es de la API dueña.
- Fechas en UTC (RFC 3339 con `Z`); las de negocio, como vinieron.
- Cada operación sensible genera su audit event **en la API dueña**, con el `correlationId` que propaga el
  gateway. El gateway no audita por su cuenta.

# Reglas de este repo
- **Sin reglas de negocio.** Si aparece un `if` sobre un estado de factura, un rol o un monto para decidir
  algo, está en el repo equivocado. Solo se permite lógica de presentación y composición: a qué API llamar,
  cómo juntar y cómo degradar.
- **El token del usuario se reenvía tal cual**, byte por byte. Prohibido un token de servicio o credenciales
  propias para leer datos de un usuario. Lo pone el cliente de salida desde el contexto verificado, nunca
  copiando el header entrante.
- **Nunca se agrega ni se lee un `organizationId`**: ni en ruta, query, body o header. Si el cliente lo manda
  por query, se borra.
- **Ninguna ruta fuera de la tabla** (`internal/domain/routes`). Nada de reverse proxy abierto. Una ruta que no
  está declarada es 404 y no llega a ninguna API. Las rutas `/internal/v1/...` de las APIs jamás se exponen.
- **Ningún header fuera de la lista blanca**, ni de ida ni de vuelta (también en `internal/domain/routes`).
- **Sin reintentos en comandos.** Solo `GET`, un reintento, ante error de transporte o 502/503/504. Quien
  reintenta un comando es el cliente, con la misma `Idempotency-Key`.
- **Ninguna llamada interna sin timeout**, y el presupuesto total de la petición siempre mayor que el timeout
  de cada API (lo valida la configuración al arrancar).
- Los Problem Details de una API se reenvían **sin cambios** (mismo status, mismo `type`). Los propios usan
  `urn:rdl:portal-gateway:problem:<código>` de `docs/propuestas/problems-portal-gateway.yaml`.
- Sin caché de datos de negocio. Si se propone una, su clave lleva organización **y** usuario, y va con un ADR.
- Arquitectura hexagonal: `internal/domain` sin infraestructura, casos de uso en `internal/app` con puertos
  definidos del lado del consumidor, clientes HTTP solo en `internal/adapters/downstream`. Una ruta nueva es
  una fila en la tabla, no otro proxy.
- Las cuatro APIs están declaradas aunque E-Invoice y Receivables no existan: sin URL responden 503
  `upstream-not-configured` y sus partes de una vista degradan (ADR 0002). **No borres esas filas.**
- No inventes un broker, eventos ni estados que no estén en el catálogo del contrato: propón y deja un `TODO`.
- No ejecutes comandos de git: el repo lo publica el usuario.
- Nunca uses la `service_role` key, producción, ni datos o certificados reales.
- Pendientes y decisiones abiertas: `docs/ESTADO.md`. Agregar ahí todo TODO nuevo.

# Antes de terminar cada tarea
```
go build ./... && go vet ./... && golangci-lint run && go test ./...
```
Luego revisa tu diff buscando: reglas de negocio infiltradas, un `organizationId` agregado o leído del cliente,
tokens de servicio en lugar del del usuario, rutas fuera de la tabla, headers reenviados sin estar en la lista
blanca, cachés sin organización y usuario en la clave, llamadas sin timeout, reintentos en comandos,
notificaciones que puedan cruzar organizaciones, secretos y errores internos expuestos en la respuesta.
