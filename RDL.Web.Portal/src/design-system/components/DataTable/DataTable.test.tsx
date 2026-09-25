import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/shared/api/types'
import { DataTable, type Column } from './DataTable'

interface Row {
  id: string
  name: string
  total: string
}
const rows: Row[] = [
  { id: '1', name: 'Café Monteazul S.A.', total: '₡452 500,00' },
  { id: '2', name: 'Soluciones Ibis S.R.L.', total: '₡113 000,00' },
]
const columns: Column<Row>[] = [
  { key: 'name', header: 'Cliente', cell: (r) => r.name },
  { key: 'total', header: 'Total', align: 'right', cell: (r) => r.total },
]
const base = { columns, rowKey: (r: Row) => r.id, label: 'Clientes', empty: <p>Sin clientes</p> }

describe('DataTable', () => {
  it('muestra filas y abre con Enter; ↓ mueve el foco', async () => {
    const onRowClick = vi.fn()
    render(<DataTable {...base} rows={rows} status="success" onRowClick={onRowClick} />)
    const [first, second] = screen.getAllByRole('row').slice(1)
    first!.focus()
    await userEvent.keyboard('{ArrowDown}')
    expect(second).toHaveFocus()
    await userEvent.keyboard('{Enter}')
    expect(onRowClick).toHaveBeenCalledWith(rows[1])
  })

  it('estado vacío', () => {
    render(<DataTable {...base} rows={[]} status="success" />)
    expect(screen.getByText('Sin clientes')).toBeInTheDocument()
  })

  it('error con código de referencia y reintento', async () => {
    const onRetry = vi.fn()
    const error = new ApiError({ status: 503, type: 'x', title: 'y', correlationId: 'abc-123' })
    render(<DataTable {...base} rows={undefined} status="error" error={error} onRetry={onRetry} />)
    expect(screen.getByText(/abc-123/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(onRetry).toHaveBeenCalled()
  })

  it('cargando marca la tabla como ocupada', () => {
    render(<DataTable {...base} rows={undefined} status="pending" />)
    expect(screen.getByRole('table', { name: 'Clientes' })).toHaveAttribute('aria-busy', 'true')
  })
})
