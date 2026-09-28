import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { OrganizationPatch } from '@/shared/api/admin-types'
import { mockDataSource } from '@/shared/api/mock'
import { resetMockAdmin } from '@/shared/api/mock/admin'
import type { DataSource } from '@/shared/api/ports'
import { ApiError } from '@/shared/api/types'
import { renderApp } from '@/test/renderApp'

afterEach(() => resetMockAdmin())

const platformProblem = (status: number, code: string, errors?: { field: string; message: string }[]) =>
  new ApiError({
    status,
    type: `urn:rdl:platform:problem:${code}`,
    title: code,
    correlationId: 'cid-7',
    errors,
  })

describe('pantalla 28 · organización', () => {
  it('manda solo lo que cambió; razón social, identificación y moneda son de solo lectura', async () => {
    const patches: OrganizationPatch[] = []
    const source: DataSource = {
      ...mockDataSource,
      organization: {
        ...mockDataSource.organization,
        update: (patch) => {
          patches.push(patch)
          return mockDataSource.organization.update(patch)
        },
      },
    }
    renderApp('/admin/organizacion', { source })
    const user = userEvent.setup()

    expect(await screen.findByText('Jurídica 3-101-900001')).toBeInTheDocument()
    expect(within(screen.getByRole('main')).getByText('Comercial Los Almendros S.A.')).toBeInTheDocument()
    expect(screen.getByText('Colones (₡)')).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: /Razón social/ })).not.toBeInTheDocument()
    const save = screen.getByRole('button', { name: 'Guardar cambios' })
    expect(save).toBeDisabled()

    await user.clear(screen.getByLabelText('Nombre comercial'))
    await user.selectOptions(screen.getByLabelText('Zona horaria'), 'America/Panama')
    await user.click(save)
    expect(await screen.findByText('Datos de la organización guardados')).toBeInTheDocument()
    expect(patches).toEqual([{ tradeName: '', timezone: 'America/Panama' }])
  })

  it('un 422 del servidor cae en su campo', async () => {
    const source: DataSource = {
      ...mockDataSource,
      organization: {
        ...mockDataSource.organization,
        update: () =>
          Promise.reject(
            platformProblem(422, 'validation', [{ field: 'phone', message: 'Teléfono inválido.' }]),
          ),
      },
    }
    renderApp('/admin/organizacion', { source })
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Teléfono'), '9')
    await user.click(screen.getByRole('button', { name: 'Guardar cambios' }))
    expect(await screen.findByText('Teléfono inválido.')).toBeInTheDocument()
  })
})

describe('pantalla 29 · sucursales', () => {
  it('valida el código antes de enviar y crea la sucursal', async () => {
    renderApp('/admin/organizacion/sucursales')
    const user = userEvent.setup()
    await screen.findByText('Escazú')
    await user.click(screen.getByRole('button', { name: 'Nueva sucursal' }))
    const form = screen.getByRole('form', { name: 'Nueva sucursal' })

    await user.type(within(form).getByLabelText(/Código/), '12')
    await user.click(within(form).getByRole('button', { name: 'Crear sucursal' }))
    expect(within(form).getByText('El código tiene 3 dígitos, por ejemplo 003.')).toBeInTheDocument()
    expect(within(form).getByText('Escriba un nombre.')).toBeInTheDocument()

    await user.clear(within(form).getByLabelText(/Código/))
    await user.type(within(form).getByLabelText(/Código/), '001')
    await user.type(within(form).getByLabelText(/Nombre/), 'Heredia')
    await user.click(within(form).getByRole('button', { name: 'Crear sucursal' }))
    expect(within(form).getByText('Ya existe una sucursal con el código 001.')).toBeInTheDocument()

    await user.clear(within(form).getByLabelText(/Código/))
    await user.type(within(form).getByLabelText(/Código/), '003')
    await user.click(within(form).getByRole('button', { name: 'Crear sucursal' }))
    expect(await screen.findByText('Heredia')).toBeInTheDocument()
    expect(screen.queryByRole('form', { name: 'Nueva sucursal' })).not.toBeInTheDocument()
  })

  it('desactiva y cambia el nombre sin tocar el código', async () => {
    renderApp('/admin/organizacion/sucursales')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Desactivar la sucursal 002' }))
    expect(await screen.findByRole('button', { name: 'Activar la sucursal 002' })).toBeInTheDocument()
    expect(screen.getByText('Inactiva')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Editar la sucursal 001' }))
    const name = screen.getByRole('textbox', { name: 'Nombre de la sucursal 001' })
    await user.clear(name)
    await user.type(name, 'Central{Enter}')
    expect(await screen.findByText('Central')).toBeInTheDocument()
    expect(screen.getByText('001')).toBeInTheDocument()
  })
})

describe('pantalla 30 · usuarios y roles', () => {
  it('nadie se cambia a sí mismo; un administrador no toca propietarios; cambia el rol de otro', async () => {
    renderApp('/admin/usuarios')
    const user = userEvent.setup()
    const table = await screen.findByRole('table', { name: 'Miembros' })
    expect(await within(table).findByText('Usted')).toBeInTheDocument()
    expect(within(table).getByRole('combobox', { name: 'Rol de María Rojas Vargas' })).toBeDisabled()
    expect(within(table).getByRole('combobox', { name: 'Rol de Andrés Quesada Mora' })).toBeDisabled()

    const sofia = within(table).getByRole('combobox', { name: 'Rol de Sofía Chaves León' })
    expect(within(sofia).queryByRole('option', { name: 'Propietario' })).not.toBeInTheDocument()
    await user.selectOptions(sofia, 'accountant')
    expect(await screen.findByText('Rol actualizado')).toBeInTheDocument()
    await waitFor(() => expect(sofia).toHaveValue('accountant'))

    await user.click(within(table).getByRole('button', { name: 'Reactivar a Diego Arias Solano' }))
    expect(
      await within(table).findByRole('button', { name: 'Suspender a Diego Arias Solano' }),
    ).toBeInTheDocument()
  })

  it('el único propietario no se puede degradar ni suspender', async () => {
    // En «Ferretería El Roble» la usuaria es propietaria: puede tocar propietarios.
    renderApp('/admin/usuarios', { orgId: 'fr' })
    const user = userEvent.setup()
    const table = await screen.findByRole('table', { name: 'Miembros' })
    const andres = await within(table).findByRole('combobox', { name: 'Rol de Andrés Quesada Mora' })
    expect(andres).toBeEnabled()
    await user.selectOptions(andres, 'admin')
    expect(
      await screen.findByText(
        'La organización debe tener al menos un propietario. Asigne otro propietario antes de cambiar este rol.',
      ),
    ).toBeInTheDocument()
    expect(andres).toHaveValue('owner')

    await user.click(within(table).getByRole('button', { name: 'Suspender a Andrés Quesada Mora' }))
    expect(
      await screen.findByText('No se puede suspender al único propietario de la organización.'),
    ).toBeInTheDocument()
  })

  it('las reglas de Platform llegan como mensaje, no como error genérico', async () => {
    const source: DataSource = {
      ...mockDataSource,
      members: {
        ...mockDataSource.members,
        update: () => Promise.reject(platformProblem(403, 'owner-required')),
      },
    }
    renderApp('/admin/usuarios', { source })
    const user = userEvent.setup()
    const table = await screen.findByRole('table', { name: 'Miembros' })
    await user.selectOptions(
      await within(table).findByRole('combobox', { name: 'Rol de Sofía Chaves León' }),
      'collector',
    )
    expect(
      await screen.findByText('Solo un propietario puede asignar o quitar el rol de propietario.'),
    ).toBeInTheDocument()
  })
})

describe('pantalla 31 · invitaciones', () => {
  it('valida el correo, avisa si ya está invitado o es miembro y muestra el enlace una sola vez', async () => {
    renderApp('/admin/usuarios/invitaciones')
    const user = userEvent.setup()
    const form = await screen.findByRole('form', { name: 'Invitar usuario' })
    const email = within(form).getByLabelText(/Correo de la persona/)

    await user.type(email, 'no-es-correo')
    await user.click(within(form).getByRole('button', { name: 'Crear invitación' }))
    expect(within(form).getByText('Escriba un correo válido.')).toBeInTheDocument()

    await user.clear(email)
    await user.type(email, 'laura.vindas@almendros.example')
    await user.click(within(form).getByRole('button', { name: 'Crear invitación' }))
    expect(within(form).getByText('Ya hay una invitación pendiente para este correo.')).toBeInTheDocument()

    await user.clear(email)
    await user.type(email, 'sofia.chaves@almendros.example')
    await user.click(within(form).getByRole('button', { name: 'Crear invitación' }))
    expect(
      await within(form).findByText('Esta persona ya es miembro de la organización.'),
    ).toBeInTheDocument()

    await user.clear(email)
    await user.type(email, 'jorge.mora@correo.example')
    await user.selectOptions(within(form).getByLabelText('Rol'), 'collector')
    await user.click(within(form).getByRole('button', { name: 'Crear invitación' }))
    expect(
      await screen.findByText('✓ Invitación creada para jorge.mora@correo.example · Cobrador'),
    ).toBeInTheDocument()
    expect(screen.getByText(/\/invitacion\/demo/)).toBeInTheDocument()
    const table = screen.getByRole('table', { name: 'Invitaciones' })
    expect(await within(table).findByText('jorge.mora@correo.example')).toBeInTheDocument()
  })

  it('un administrador no puede invitar propietarios', async () => {
    renderApp('/admin/usuarios/invitaciones')
    const role = within(await screen.findByRole('form', { name: 'Invitar usuario' })).getByLabelText('Rol')
    expect(within(role).queryByRole('option', { name: 'Propietario' })).not.toBeInTheDocument()
  })

  it('sin token en la respuesta (reintento de la misma alta) no inventa un enlace', async () => {
    const source: DataSource = {
      ...mockDataSource,
      invitations: {
        ...mockDataSource.invitations,
        create: async (input) => ({
          id: 'i9',
          ...input,
          status: 'pending',
          createdAt: '2026-09-24T15:00:00Z',
          expiresAt: '2026-10-01T15:00:00Z',
        }),
      },
    }
    renderApp('/admin/usuarios/invitaciones', { source })
    const user = userEvent.setup()
    const form = await screen.findByRole('form', { name: 'Invitar usuario' })
    await user.type(within(form).getByLabelText(/Correo de la persona/), 'otra@correo.example')
    await user.click(within(form).getByRole('button', { name: 'Crear invitación' }))
    expect(await screen.findByText(/su enlace solo se mostró la primera vez/)).toBeInTheDocument()
    expect(screen.queryByText(/\/invitacion\//)).not.toBeInTheDocument()
  })

  it('revoca con confirmación', async () => {
    const revoke = vi.spyOn(mockDataSource.invitations, 'revoke')
    renderApp('/admin/usuarios/invitaciones')
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Revocar la invitación de laura.vindas@almendros.example' }),
    )
    const dialog = screen.getByRole('dialog', { name: '¿Revocar la invitación?' })
    await user.click(within(dialog).getByRole('button', { name: 'Revocar' }))
    expect(await screen.findByText('Invitación revocada')).toBeInTheDocument()
    expect(revoke).toHaveBeenCalledWith('i1')
    const table = screen.getByRole('table', { name: 'Invitaciones' })
    await waitFor(() =>
      expect(
        within(table).queryByRole('button', { name: /Revocar la invitación de laura/ }),
      ).not.toBeInTheDocument(),
    )
    revoke.mockRestore()
  })
})

describe('pantalla 32 · exportar auditoría', () => {
  it('sin API de auditoría no simula una descarga', async () => {
    renderApp('/admin/auditoria')
    expect(await screen.findByText('Todavía no disponible')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Descargar CSV' })).toBeDisabled()
  })

  it('solo propietarios y administradores entran', async () => {
    renderApp('/admin/auditoria', { orgId: 'si' })
    expect(await screen.findByText('No tiene acceso a esta sección')).toBeInTheDocument()
  })
})
