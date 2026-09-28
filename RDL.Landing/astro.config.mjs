import sitemap from '@astrojs/sitemap'
import { defineConfig } from 'astro/config'
import { loadEnv } from 'vite'

const env = loadEnv(process.env.NODE_ENV ?? 'production', process.cwd(), '')
const site = env.PUBLIC_SITE_URL || 'https://www.rdl.example'

// Páginas que no van al sitemap: los borradores legales (noindex hasta la revisión legal) y la 404.
const NOT_INDEXED = ['/terminos/', '/privacidad/', '/404/']

export default defineConfig({
  site,
  trailingSlash: 'ignore',
  build: {
    // CSP estricta (sin 'unsafe-inline'): ningún estilo en línea, todo en archivos.
    inlineStylesheets: 'never',
  },
  vite: {
    // Astro incrusta en línea los scripts y archivos chicos: con 0, todo sale en archivos y la CSP no necesita
    // 'unsafe-inline' ni hashes.
    build: { assetsInlineLimit: 0 },
  },
  integrations: [
    sitemap({
      filter: (page) => !NOT_INDEXED.some((path) => new URL(page).pathname === path),
    }),
  ],
})
