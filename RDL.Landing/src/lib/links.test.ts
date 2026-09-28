import assert from 'node:assert/strict'
import { test } from 'node:test'
import { portalUrl, utmFrom, whatsappUrl, withUtm } from './links.ts'

test('URL del portal con o sin barras', () => {
  assert.equal(portalUrl('https://app.rdl.example', '/ingresar'), 'https://app.rdl.example/ingresar')
  assert.equal(portalUrl('https://app.rdl.example/', 'ingresar'), 'https://app.rdl.example/ingresar')
})

test('solo los UTM de la visita, sin vacíos', () => {
  assert.deepEqual(utmFrom('?utm_source=facebook&utm_campaign=lanzamiento&x=1&utm_medium='), [
    ['utm_source', 'facebook'],
    ['utm_campaign', 'lanzamiento'],
  ])
  assert.deepEqual(utmFrom(''), [])
})

test('los UTM se agregan al enlace del portal sin pisar los que ya trae', () => {
  const href = withUtm('https://app.rdl.example/ingresar?utm_source=boton', [
    ['utm_source', 'facebook'],
    ['utm_medium', 'social'],
  ])
  assert.equal(href, 'https://app.rdl.example/ingresar?utm_source=boton&utm_medium=social')
})

test('WhatsApp: solo dígitos y mensaje codificado; sin número no hay enlace', () => {
  assert.equal(
    whatsappUrl('+506 0000-0000', 'Hola, ¿me dan información?'),
    'https://wa.me/50600000000?text=Hola%2C%20%C2%BFme%20dan%20informaci%C3%B3n%3F',
  )
  assert.equal(whatsappUrl('', 'x'), null)
  assert.equal(whatsappUrl(undefined, 'x'), null)
})
