# 0006 · Numeración visible y huecos

- **Fecha:** 2026-09-25 · **Estado:** Aceptado (informe 0001 §4.5, aprobado)
- **Alcance:** el número que ve el negocio (`billing.invoices.number`, `FAC-00000001`). El consecutivo fiscal y la
  clave numérica de Hacienda son de E-Invoice.

## Formato y alcance

- **Número visible = prefijo + número con ceros a la izquierda hasta 8 dígitos** (`FAC-00000001`). Pasado 99 999 999
  crece sin truncar. Prefijo de hasta 10 caracteres (`[A-Za-z0-9._/-]`, puede ser vacío).
- **Qué secuencia se usa al emitir** (`numbering.Choose`): la del tipo de documento y la sucursal del documento; si no
  existe, la de la organización (sin sucursal); si tampoco existe, se crea una de la organización (prefijo vacío,
  desde 1).
- **Sin prefijos repetidos dentro de un tipo de documento.** `invoices_number_uk` es `(organización, tipo, número)`,
  sin sucursal: dos sucursales con el mismo prefijo generarían el mismo número. `PUT /v1/document-sequences/{tipo}`
  responde 409 `conflict`. El mismo prefijo en otro tipo sí se permite.
- **Configurar solo lo no usado.** Prefijo y número inicial se cambian solo si la secuencia nunca asignó un número
  (409 `sequence-in-use`). Para saberlo, la migración `00002` agrega `last_assigned_number` (NULL = nunca usada): con
  `next_number` solo no se distingue "configurada para arrancar en 1000" de "ya emitió 999".

## Concurrencia

Todo lo que toca las secuencias de un tipo de documento en una organización (configurar y, en el incremento 8,
asignar) toma primero `pg_advisory_xact_lock` sobre `(organización, tipo)` y después `SELECT … FOR UPDATE` de la
secuencia. El candado asesor cubre lo que el `FOR UPDATE` no puede: validar el prefijo contra secuencias que todavía
no existen y crear la secuencia la primera vez. Ambos se liberan al terminar la transacción.

Costo aceptado: las emisiones del mismo tipo en la misma organización se serializan. Para una PYME es irrelevante;
si algún día no lo fuera, la salida es una secuencia por sucursal (que ya existe).

## Huecos

**No hay huecos por rollback.** El número se asigna dentro de la transacción de la emisión (`Sequence.Assign` y
`UPDATE` de la secuencia junto con la factura, el historial, el audit y el outbox). Si cualquier cosa falla, se
revierte todo, incluido el avance de la secuencia: el número vuelve a estar disponible para la siguiente emisión.
Por eso no se usa una `SEQUENCE` de Postgres, que no se revierte y dejaría huecos.

**Sí puede haber huecos por decisiones del usuario**, y son legítimos: configurar la secuencia para empezar en 1000
(nunca hubo 1…999). Los borradores no consumen números (se numeran al emitir), así que descartar un borrador no deja
hueco. La anulación (F5) conserva el número del documento anulado.
