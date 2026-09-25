import { fileURLToPath, URL } from 'node:url'
import react from '@vitejs/plugin-react'
import { loadEnv, type Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

/**
 * CSP estricta del build (ADR 0005): la sesión vive en localStorage, así que lo que la protege es que solo se
 * ejecute el código del portal y que solo se conecte al Portal Gateway y a Supabase. Sin scripts ni estilos en
 * línea: el build no genera ninguno. No se aplica en `vite dev` (su recarga en caliente usa scripts en línea).
 *
 * `frame-ancestors` no funciona en un <meta>: va como cabecera del servidor que sirva el estático
 * (TODO(despliegue): Dockerfile con nginx o Caddy, P8b paso 7).
 */
function origin(url: string | undefined): string | undefined {
  try {
    return url ? new URL(url).origin : undefined
  } catch {
    return undefined
  }
}

export function contentSecurityPolicy(env: Record<string, string>): string {
  const connect = ["'self'"]
  if (env.VITE_DATA_SOURCE === 'gateway') {
    const gateway = origin(env.VITE_GATEWAY_URL ?? 'http://localhost:8090')
    const supabase = origin(env.VITE_SUPABASE_URL)
    if (gateway) connect.push(gateway)
    if (supabase) connect.push(supabase, supabase.replace(/^http/, 'ws'))
  }
  return [
    "default-src 'self'",
    "script-src 'self'",
    "style-src 'self'",
    "img-src 'self' data:",
    "font-src 'self'",
    `connect-src ${connect.join(' ')}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join('; ')
}

function cspPlugin(env: Record<string, string>): Plugin {
  return {
    name: 'rdl-csp',
    apply: 'build',
    transformIndexHtml: () => [
      {
        tag: 'meta',
        attrs: { 'http-equiv': 'Content-Security-Policy', content: contentSecurityPolicy(env) },
        injectTo: 'head-prepend',
      },
    ],
  }
}

export default defineConfig(({ mode }) => ({
  plugins: [react(), cspPlugin(loadEnv(mode, process.cwd(), 'VITE_'))],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: { port: 5173, strictPort: true },
  build: {
    rolldownOptions: {
      output: {
        // Bibliotecas en paquetes propios: cambian poco y el navegador las conserva en caché entre versiones.
        codeSplitting: {
          groups: [
            { name: 'react', test: /node_modules[\\/](react|react-dom|scheduler|react-router)[\\/]/ },
            { name: 'ui', test: /node_modules[\\/](@radix-ui|@floating-ui|lucide-react)[\\/]/ },
            { name: 'data', test: /node_modules[\\/](@tanstack|big\.js|zod|react-hook-form)[\\/]/ },
          ],
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: false,
    setupFiles: ['./src/test/setup.ts'],
    css: { modules: { classNameStrategy: 'non-scoped' } },
    include: ['src/**/*.test.{ts,tsx}'],
  },
}))
