import { act, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { renderApp } from '@/test/renderApp'

// T-F006 (ADR-017): all data-router routes expose their provisional accessible heading.
describe('application routes', () => {
  it.each([
    ['/login', 'Ingresar'],
    ['/signup', 'Registro'],
    ['/forgot-password', 'Olvidé mi contraseña'],
    ['/reset-password#token=example', 'Restablecer contraseña'],
    ['/verify-email#token=example', 'Confirmá tu email'],
    ['/accept-invitation#token=example', 'Aceptar invitación'],
    ['/', 'Panel inicial'],
    ['/settings', 'Ajustes'],
    ['/settings/company', 'Datos de la empresa'],
    ['/settings/users', 'Usuarios'],
    ['/settings/users/invite', 'Invitar usuario'],
    ['/does-not-exist', 'No encontramos esta página.'],
  ])('shows the heading for %s', async (path, heading) => {
    renderApp([path])
    expect(
      await screen.findByRole('heading', { level: 1, name: heading }),
    ).toBeVisible()
  })
  it('navigates from home to settings and back without console errors', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const { router } = renderApp(['/'])
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Panel inicial' }),
    ).toBeVisible()
    await act(async () => router.navigate('/settings'))
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Ajustes' }),
    ).toBeVisible()
    await act(async () => router.navigate(-1))
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Panel inicial' }),
    ).toBeVisible()
    expect(error).not.toHaveBeenCalled()
  })
})
