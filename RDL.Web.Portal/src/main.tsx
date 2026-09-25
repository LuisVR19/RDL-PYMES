import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@/design-system/global.css'
import { App } from '@/app/App'

const root = document.getElementById('root')
if (!root) throw new Error('Falta el elemento #root en index.html')

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
