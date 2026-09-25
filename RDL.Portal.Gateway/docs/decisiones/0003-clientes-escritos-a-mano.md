# 0003 · Clientes escritos a mano y reintentos

**Estado:** aceptada (incremento 1–4).
**Fecha:** 2026-09-24.

## Contexto

El prompt P7 pide decidir si los clientes de las APIs se escriben a mano o se generan desde los OpenAPI del
repo de contratos (por ejemplo con `oapi-codegen`).

## Decisión

**A mano**, por ahora.

El paso directo —54 de las 55 rutas— **no decodifica nada**: copia método, cuerpo, headers declarados y
respuesta. Un cliente generado no aporta nada ahí, y traería tipos para cada operación de cuatro APIs.

Las composiciones sí decodifican, pero solo los pocos campos de `openapi/bff-internal.yaml`: ocho del resumen
de factura, tres del estado fiscal y cinco del saldo. Son los DTO de `internal/adapters/downstream/readers.go`,
y se ignora a propósito todo lo demás que venga: el gateway no lo necesita y no lo va a reinterpretar.

**Revisar esta decisión cuando** existan E-Invoice y Receivables y los listados enriquecidos (incremento 5)
hagan falta más campos. Ahí sí conviene generar, al menos, los tipos de las rutas internas.

## Reintentos

- **Solo lecturas** (`GET`), **un** reintento, con 50 ms de espera, ante un error de transporte o un
  502/503/504. Esos tres códigos significan que la API no procesó nada.
- **Nunca un comando.** Un `POST /v1/invoices/{id}/issue` reintentado por el gateway podría emitir dos
  facturas. Quien reintenta es el cliente, con la misma `Idempotency-Key`, que es lo único que de verdad
  garantiza no duplicar. Hay una prueba dedicada a la emisión.
- Un 409 o un 422 **no** se reintentan: la API sí procesó la petición y decidió.

## Timeouts y presupuesto

- Un timeout por API (`UPSTREAM_TIMEOUT`, con override por servicio), que cubre también leer el cuerpo:
  es `http.Client.Timeout`, no solo la conexión. **Ninguna llamada interna queda sin timeout.**
- Un presupuesto total por petición (`UPSTREAM_BUDGET`). La configuración **exige** que el presupuesto supere
  al timeout más largo: si no, una composición se cortaría antes de alcanzar a degradar, que es justo lo que
  no queremos.
- Concurrencia acotada (`errgroup` con límite 3) y cancelación por contexto cuando el cliente se va.

## Caché

Ninguna de datos de negocio en V1. El JWKS sí (lo cachea el adapter de auth, igual que Platform).
Si algún día se propone una, su clave lleva **organización y usuario**, y viene con su propio ADR.
