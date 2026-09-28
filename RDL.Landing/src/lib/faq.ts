import { config } from './config'
import { content, fill } from './content'

/** Las preguntas frecuentes con los datos de contacto ya puestos: las usan la sección y el JSON-LD `FAQPage`. */
export function faqItems(): { q: string; a: string }[] {
  const vars = {
    email: config.contact.email,
    phone: config.contact.phoneDisplay,
    hours: config.contact.hours,
  }
  return content.home.faq.items.map((it) => ({ q: it.q, a: fill(it.a, vars) }))
}

export function faqJsonLd(): object {
  return {
    '@context': 'https://schema.org',
    '@type': 'FAQPage',
    mainEntity: faqItems().map((it) => ({
      '@type': 'Question',
      name: it.q,
      acceptedAnswer: { '@type': 'Answer', text: it.a },
    })),
  }
}
