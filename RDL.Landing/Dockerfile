# Imagen de la landing: construye el estático con los PUBLIC_* del ambiente y lo sirve con Caddy.
#   docker build --build-arg PUBLIC_SITE_URL=https://www.dominio.cr --build-arg … -t rdl-landing .
#   (un --build-arg por cada PUBLIC_* de .env.example; el .env no entra a la imagen: un dato sin valor no se muestra)
#   docker run -p 8080:8080 -e UMAMI_ORIGIN=https://analytics.dominio.cr rdl-landing

FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG PUBLIC_SITE_URL PUBLIC_PORTAL_URL PUBLIC_PORTAL_LOGIN_PATH PUBLIC_SIGNUP_ENABLED PUBLIC_PORTAL_SIGNUP_PATH \
    PUBLIC_CONTACT_EMAIL PUBLIC_CONTACT_WHATSAPP PUBLIC_CONTACT_PHONE_DISPLAY PUBLIC_CONTACT_HOURS \
    PUBLIC_CONTACT_RESPONSE_TIME PUBLIC_LEGAL_NAME PUBLIC_LEGAL_ID PUBLIC_LEGAL_ADDRESS PUBLIC_PRIVACY_EMAIL \
    PUBLIC_UMAMI_SRC PUBLIC_UMAMI_WEBSITE_ID
RUN npm run build

FROM caddy:2-alpine
ENV UMAMI_ORIGIN=""
COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/dist /srv
EXPOSE 8080
