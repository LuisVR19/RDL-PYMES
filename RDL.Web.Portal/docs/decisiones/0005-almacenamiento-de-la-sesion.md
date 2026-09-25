# 0005 · Almacenamiento de la sesión: localStorage con CSP estricta

- Estado: aceptada · 2026-09-25 (decisión de Luis Valverde, P8b paso 4.2)

## Contexto
`supabase-js` guarda la sesión (access token y refresh token) en `localStorage` por defecto. El prompt P8b pide
decidir si se mantiene, con una CSP estricta que reduzca el riesgo de XSS, o si se usa memoria con refresh.

Cualquier almacenamiento que JavaScript pueda leer queda expuesto a un XSS; la diferencia entre las opciones es
cuánto dura la sesión y cuánta fricción tiene quien factura todo el día:

| Opción | Recarga | Pestaña nueva | Exposición |
|---|---|---|---|
| **localStorage** | sigue la sesión | sigue la sesión | mientras la sesión viva |
| sessionStorage | sigue la sesión | pide login | mientras la pestaña viva |
| Solo memoria | pide login | pide login | mientras la página viva |

Lo único que saca el token del alcance de JavaScript es una cookie `HttpOnly` emitida por el servidor, y eso exige
que el Portal Gateway maneje la sesión: fuera de alcance en V1.

## Decisión
- Se mantiene **`localStorage`** (valor por defecto de supabase-js), con refresco automático.
- El build lleva una **CSP estricta** en un `<meta>` (`vite.config.ts`): `script-src 'self'`, `style-src 'self'`,
  `object-src 'none'`, `base-uri 'self'`, `form-action 'self'` y `connect-src` limitado al Portal Gateway y a
  Supabase (https y wss). El build no genera scripts ni estilos en línea; una e2e comprueba que la CSP está activa
  y que el navegador no bloquea nada del portal.
- Al cerrar sesión, o si la sesión se pierde (logout en otra pestaña, refresh vencido), se vacía **toda** la caché
  de TanStack Query. Al iniciar sesión también: otro usuario pudo haber usado el navegador.
- `?volver=` solo acepta rutas internas: un enlace no puede sacar al usuario del portal después del login.

## Pendiente
- `frame-ancestors 'none'`, `X-Content-Type-Options` y `Referrer-Policy` no funcionan en un `<meta>`: van como
  cabeceras del servidor que sirva el estático (Dockerfile, P8b paso 7). Hasta entonces el portal no tiene
  protección contra clickjacking.
- En `vite dev` no hay CSP (la recarga en caliente usa scripts en línea).
