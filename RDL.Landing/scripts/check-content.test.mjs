import assert from 'node:assert/strict'
import { test } from 'node:test'
import { findProblems } from './check-content.mjs'

test('marcadores sin reemplazar', () => {
  assert.equal(findProblems('Escríbanos a <<ventas@…>>')[0]?.why, 'marcador sin reemplazar')
})

test('afirmaciones prohibidas sobre Hacienda', () => {
  assert.equal(findProblems('Sistema certificado por Hacienda').length, 1)
  assert.equal(findProblems('Aprobado por el Ministerio de Hacienda').length, 1)
  assert.equal(findProblems('Proveedor oficial de Hacienda').length, 1)
})

test('la negación explícita sí se permite', () => {
  assert.deepEqual(findProblems('RDL no está certificado ni avalado por Hacienda.'), [])
})

test('cifras de clientes inventadas', () => {
  assert.equal(findProblems('Más de +1000 empresas confían en nosotros').length, 1)
})

test('el certificado de firma no es una afirmación prohibida', () => {
  assert.deepEqual(findProblems('Suba su certificado de firma y conecte su usuario de Hacienda.'), [])
})
