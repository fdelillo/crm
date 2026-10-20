import { act, screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { server } from '@/test/msw/server'
import { buildSessionInfo, buildTenant, problem } from '@/test/fixtures'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderApp } from '@/test/renderApp'

// T-F006 (ADR-017), T-F402–T-F404: data-router routes expose their accessible headings.
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
    ['/login', 'Ingresá a tu cuenta'],
    ['/signup', 'Creá tu cuenta'],
    ['/forgot-password', 'Recuperá tu contraseña'],
    ['/reset-password#token=example', 'Creá una contraseña nueva'],
    ['/verify-email#token=example', 'Confirmá tu email'],
    ['/accept-invitation#token=example', 'Aceptar invitación'],
    ['/', 'Hola, Persona de prueba'],
    ['/settings', 'Ajustes'],
    ['/settings/company', 'Datos de la empresa'],
    ['/settings/users', 'Usuarios'],
    ['/settings/users/invite', 'Invitar usuario'],
    ['/does-not-exist', 'No encontramos esta página.'],
  ])(
    'shows the heading for %s',
    async (path, heading) => {
      session(path === '/' || path.startsWith('/settings'))
      renderApp([path])
      expect(
        await screen.findByRole('heading', { level: 1, name: heading }),
      ).toBeVisible()
    },
    5_000,
  )
  it('navigates from home to settings and back without console errors', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    session(true)
    const { router } = renderApp(['/'])
    expect(
      await screen.findByRole('heading', {
        level: 1,
        name: 'Hola, Persona de prueba',
      }),
    ).toBeVisible()
    await act(async () => router.navigate('/settings'))
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Ajustes' }),
    ).toBeVisible()
    await act(async () => router.navigate(-1))
    expect(
      await screen.findByRole('heading', {
        level: 1,
        name: 'Hola, Persona de prueba',
      }),
    ).toBeVisible()
    expect(error).not.toHaveBeenCalled()
  })
})
