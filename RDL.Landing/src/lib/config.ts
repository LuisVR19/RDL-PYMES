import { portalUrl, whatsappUrl } from './links'
import { contact } from './content'

/**
 * Configuración del sitio desde las variables `PUBLIC_*` (ver `.env.example`). Se lee al construir: cada ambiente
 * construye con sus valores. Nada de esto es secreto (todo termina en el HTML público).
 */
const env = import.meta.env

const portal = env.PUBLIC_PORTAL_URL || 'http://localhost:5173'

export const config = {
  siteUrl: env.PUBLIC_SITE_URL || 'https://www.rdl.example',
  loginUrl: portalUrl(portal, env.PUBLIC_PORTAL_LOGIN_PATH || '/ingresar'),
  /** null = el portal todavía no tiene registro: «Crear cuenta» se ve como «Próximamente». */
  signupUrl:
    env.PUBLIC_SIGNUP_ENABLED === 'true'
      ? portalUrl(portal, env.PUBLIC_PORTAL_SIGNUP_PATH || '/registro')
      : null,
  contact: {
    email: env.PUBLIC_CONTACT_EMAIL || '',
    phoneDisplay: env.PUBLIC_CONTACT_PHONE_DISPLAY || '',
    hours: env.PUBLIC_CONTACT_HOURS || '',
    responseTime: env.PUBLIC_CONTACT_RESPONSE_TIME || '',
    /** null = sin número configurado: los botones de WhatsApp no se muestran. */
    whatsappUrl: whatsappUrl(env.PUBLIC_CONTACT_WHATSAPP, contact.whatsappMessage),
  },
  legal: {
    name: env.PUBLIC_LEGAL_NAME || '',
    id: env.PUBLIC_LEGAL_ID || '',
    address: env.PUBLIC_LEGAL_ADDRESS || '',
    privacyEmail: env.PUBLIC_PRIVACY_EMAIL || env.PUBLIC_CONTACT_EMAIL || '',
  },
  umami:
    env.PUBLIC_UMAMI_SRC && env.PUBLIC_UMAMI_WEBSITE_ID
      ? { src: env.PUBLIC_UMAMI_SRC, websiteId: env.PUBLIC_UMAMI_WEBSITE_ID }
      : null,
}
