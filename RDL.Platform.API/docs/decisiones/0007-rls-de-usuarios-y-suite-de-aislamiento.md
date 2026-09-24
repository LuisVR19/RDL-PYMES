# 0007 · RLS de `core.users` y suite de aislamiento

- **Fecha:** 2026-09-24 · **Estado:** Aceptado (revisar en equipo los puntos marcados con ⚠)

## Contexto

La política `users_platform` de `core.users` era `using (true) with check (true)`: con la sesión de cualquier
organización, `platform_app` podía leer, crear y modificar a todos los usuarios. El informe 0001 (decisión P7)
dejó el endurecimiento para el incremento 8.

El problema es que la API necesita buscar a un usuario por su `sub` **antes** de conocer su id (en el login, en el alta
perezosa y al resolver la membresía), así que no hay sesión que una política pueda usar.

## Decisión

1. **Políticas nuevas** (migración `00005`, expand):
   - `users_self` (SELECT) y `users_self_update` (UPDATE): solo la fila de `app.current_user_id`.
   - `users_org_members` (SELECT): usuarios con membresía en la organización activa. La subconsulta pasa por la RLS de
     `organization_users`, así que no amplía nada.
   - Sin políticas de INSERT ni DELETE.
2. **Funciones `security definer`** con dueño `platform_migrator` y `search_path = ''`, el mismo patrón que el hook
   de tokens. `EXECUTE` solo para `platform_app`:
   - `core.find_user_by_subject(text)` devuelve la fila del sujeto (proveedor `supabase`).
   - `core.provision_user(text, text, text)` hace el alta idempotente (`on conflict do nothing`). Un email repetido
     sigue fallando por `users_email_uk`, que la API responde con 409.

   Ambas devuelven `setof core.users` para que sqlc tipee las consultas con las columnas de la tabla.
3. **Contract** (migración `00006`): se elimina `users_platform`. Se aplicó **después** de desplegar el código que usa
   las funciones. Con el binario anterior y `00006` aplicada, el alta de usuarios falla: el orden importa.
4. `cmd/migrate up-by-one` aplica una sola migración, para poder desplegar entre expand y contract.

## Suite de aislamiento (`tests/isolation`, `make test-isolation`)

- Usa el router real (`internal/wiring`, el mismo que usa `cmd/api`) y los adapters reales de Postgres. Solo simula la
  verificación del JWT, porque la suite no puede firmar tokens de Supabase. La membresía se revalida en la base.
- Base: `TEST_DATABASE_URL` (y `TEST_DB_PASSWORD`, opcional) o el `.env` de la API. Se niega a correr con un rol
  que salte RLS. Sin base, se salta con aviso.
- ⚠ **Desviación del prompt: no usa testcontainers.** El schema completo (auth, shared, audit, billing, fiscal,
  receivables) lo crea `database-platform`, no este repo, así que un Postgres vacío no sirve. Cuando
  `database-platform` publique una imagen o un dump del schema, se puede agregar.
- No borra datos: cada prueba crea sus propias organizaciones y usuarios con sujetos `isolation-<uuid>`. En dev se
  acumulan filas `Isolation … S.A.` (unas 11 organizaciones por corrida).
- Cubre los 6 criterios: endpoints cruzados (404/403, y B queda intacto), SELECT directo en todas las tablas con
  `organization_id` legibles, FK compuesta cruzada (23503) y escrituras con `organization_id` ajeno (42501), ninguna
  escritura en billing/fiscal/receivables ni UPDATE/DELETE en audit, token sin `org_id` o con membresía suspendida, y
  `organization_id` en body, query o header.

## Consecuencias

- ⚠ `core.provision_user` deja que `platform_app` cree usuarios con cualquier `sub`. Es lo mismo que permitía
  `users_platform`, pero ahora solo por esa vía. El `sub` siempre sale del JWT verificado.
- Un usuario sin membresías solo se ve a sí mismo. El hook de tokens no cambia (ya era `security definer`).
