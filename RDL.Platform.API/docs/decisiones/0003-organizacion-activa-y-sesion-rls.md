# 0003 · Organización activa y sesión RLS

- **Fecha:** 2026-09-23 · **Estado:** Aceptado

## Contexto

El prompt P3 proponía `app.organization_id`. La base ya define `shared.current_organization_id()` sobre `app.current_organization_id` y `shared.current_user_id()` sobre `app.current_user_id`, y todas las políticas de las cuatro APIs los usan.

## Decisión

1. **Variables de sesión:** cada transacción ejecuta, antes de cualquier consulta,
   `select set_config('app.current_organization_id', $1, true), set_config('app.current_user_id', $2, true)`.
   `true` equivale a `SET LOCAL`: el valor muere con la transacción, lo que es obligatorio con Supavisor en modo transacción.
2. **`app.current_user_id` es `core.users.id`**, no el `sub` de Supabase.
3. **Organización activa:** se guarda en `core.users.active_organization_id` (migración 00002). `PUT /v1/me/active-organization` la cambia solo si hay una membresía activa.
4. **Claims:** el hook `core.custom_access_token_hook` (migración 00003) emite `org_id` y `org_roles`. Si no hay membresía activa, el token sale sin ellos.
5. **El JWT no es la autoridad final:** el middleware revalida membresía y rol contra la BD en cada request, con una caché de 30 s como máximo (`AUTH_MEMBERSHIP_CACHE_TTL`, tope de 60 s).
6. **Un rol por membresía en V1.** La BD admite N (`core.organization_user_roles`) y la API responde `roles: []` para no romper el contrato cuando se habiliten varios. Mientras tanto, escribir un rol reemplaza el anterior.

## Consecuencias

- La organización **nunca** sale del body, la query, un header ni la ruta: solo de `org_id` del token verificado **y** de una membresía activa en BD.
- Suspender a un miembro surte efecto en ≤ TTL de la caché, sin esperar a que expire el JWT.
- Paso manual por ambiente: activar el hook en Dashboard → Authentication → Hooks.
