import { act, screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { server } from '@/test/msw/server'
import { buildSessionInfo, buildTenant, problem } from '@/test/fixtures'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderApp } from '@/test/renderApp'

// T-F006 (ADR-017): all data-router routes expose their provisional accessible heading.
describe('application routes', () => {
  beforeEach(() =>
    server.use(
      http.get('*/api/v1/tenant', () => HttpResponse.json(buildTenant())),
    ),
  )
  function session(authenticated: boolean) {
    server.use(
      http.get('*/api/v1/me', () =>
        authenticated
          ? HttpResponse.json(buildSessionInfo())
          : HttpResponse.json(problem('unauthenticated', 401), {
              status: 401,
              headers: { 'Content-Type': 'application/problem+json' },
            }),
      ),
    )
  }
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
    session(path === '/' || path.startsWith('/settings'))
    renderApp([path])
    expect(
      await screen.findByRole('heading', { level: 1, name: heading }),
    ).toBeVisible()
  })
  it('navigates from home to settings and back without console errors', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    session(true)
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
