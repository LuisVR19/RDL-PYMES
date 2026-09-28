import type { APIRoute } from 'astro'

/** robots.txt con el sitemap del dominio configurado. Los borradores legales se excluyen con `noindex`. */
export const GET: APIRoute = ({ site }) => {
  const sitemap = new URL('/sitemap-index.xml', site).toString()
  return new Response(`User-agent: *\nAllow: /\n\nSitemap: ${sitemap}\n`, {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  })
}
