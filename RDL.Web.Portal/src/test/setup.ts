import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Sin `globals` de Vitest, Testing Library no desmonta solo entre pruebas.
afterEach(() => cleanup())
