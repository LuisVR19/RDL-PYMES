# 0003 · Capa de datos por puertos y escenarios de revisión

- Estado: aceptada · 2026-09-24

## Contexto
En esta etapa no hay cableado a la API, pero las pantallas deben poder revisarse en todos sus estados y, más
adelante, conectarse al Portal Gateway sin reescribirse.

## Decisión
- Las pantallas solo conocen **puertos** (`src/shared/api/ports.ts`), obtenidos con `useDataSource()`. Nunca `fetch`.
- Hoy existe un solo adaptador, `mock/`, con datos ficticios de `src/mocks`. En la etapa de cableado se agrega
  `gateway/` (cliente generado desde el OpenAPI del gateway) y `VITE_DATA_SOURCE` elige.
- El adaptador simulado lee un **escenario** global (`scenario.ts`): `ok`, `loading` (no responde nunca), `empty`,
  `error` (Problem Details con código de referencia) y `partial` (fallan los bloques secundarios). Latencia y estado
  del tiempo real también son configurables. Se manejan desde la barra de revisión, que solo existe con `mock`.
- Los datos de sesión (usuario, organizaciones) no siguen el escenario, para que un «error» de lista no tumbe el
  armazón.
- Aislamiento por organización: al cambiar de organización se eliminan todas las consultas cuya clave no empieza
  por `'session'`; las claves de datos incluyen el id de la organización.

## Alternativas descartadas
- **MSW** en esta etapa: simula HTTP, pero sin contrato OpenAPI del gateway todavía obligaría a inventar URLs y
  formas que luego cambiarían. Se reconsidera en la etapa de cableado para pruebas de integración del adaptador.
