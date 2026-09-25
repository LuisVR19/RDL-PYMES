# 0004 · oxlint en lugar de ESLint; catálogo propio en lugar de Storybook

- Estado: aceptada · 2026-09-24

## oxlint
- Cubre las reglas que importan aquí (hooks de React, jsx-a11y, import, unicorn, TypeScript) con un solo binario,
  sin configuración de parser, en milisegundos.
- `no-restricted-globals: parseFloat` bloquea la puerta más común a montos como `number`.
- Se desactiva `jsx-a11y/prefer-tag-over-role`: pide `<output>` en lugar de `role="status"` y `<search>` en lugar de
  `role="search"`, que no mejoran la accesibilidad y tienen soporte desigual. La accesibilidad real se verifica con
  axe en Playwright.
- Si más adelante hace falta una regla que solo existe en ESLint, se agrega ESLint solo para esa regla.

## Catálogo `/_catalogo`
- Storybook duplicaría la configuración de Vite, los proveedores y los tokens. En esta etapa basta una ruta interna,
  solo con datos simulados, que muestra cada componente en sus variantes y estados con los mismos tokens del
  producto, e incluye el índice de pantallas. axe la analiza en e2e.
- Se reconsidera Storybook si el sistema de diseño se comparte con otro producto (por ejemplo, la landing).
