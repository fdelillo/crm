import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { buildSessionInfo, buildTenant, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'

const base = 'https://localhost:5173/api/v1'
const session = buildSessionInfo()
const unauthenticated = () =>
  HttpResponse.json(problem('unauthenticated', 401), {
    status: 401,
    headers: { 'Content-Type': 'application/problem+json' },
  })

beforeEach(() =>
  server.use(
    http.get(`${base}/me`, () => HttpResponse.json(session)),
    http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
  ),
)

describe('logout navigation', () => {
  // T-F303: settings logout clears server-state cache, replaces to logged-out login, and requires /me again.
  it('logs out from Settings and does not restore the old session on Back', async () => {
    let loggedOut = false
    let sessionReads = 0
    server.use(
      http.get(`${base}/me`, () => {
        sessionReads += 1
        return loggedOut ? unauthenticated() : HttpResponse.json(session)
      }),
      http.post(`${base}/auth/logout`, () => {
        loggedOut = true
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const { router, queryClient } = renderApp(['/settings'])
    await screen.findByRole('heading', { name: 'Ajustes' })
    await waitFor(() =>
      expect(queryClient.getQueryData(['tenant'])).toBeDefined(),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Cerrar sesión' }))
    expect(await screen.findByText('Cerraste sesión.')).toBeVisible()
    expect(router.state.location.pathname).toBe('/login')
    expect(router.state.location.search).toBe('?reason=logged_out')
    expect(router.state.historyAction).toBe('REPLACE')
    expect(queryClient.getQueryData(['tenant'])).toBeUndefined()

    await router.navigate('/')
    await waitFor(() => expect(sessionReads).toBeGreaterThanOrEqual(2))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  })

  // T-F303: the desktop user menu exposes the same logout action.
  it('offers logout from the desktop user menu', async () => {
    const logout = vi.fn(() => new HttpResponse(null, { status: 204 }))
    server.use(http.post(`${base}/auth/logout`, logout))
    renderApp(['/'])
    const menu = await screen.findByRole('button', {
      name: /Persona de prueba · Administrador/,
    })
    await userEvent.click(menu)
    await userEvent.click(
      await screen.findByRole('menuitem', { name: 'Cerrar sesión' }),
    )
    await waitFor(() => expect(logout).toHaveBeenCalledOnce())
    expect(await screen.findByText('Cerraste sesión.')).toBeVisible()
  })

  // T-F303: offline logout keeps the session and reports the designed toast.
  it('stays in Settings and keeps session after logout loses network', async () => {
    server.use(http.post(`${base}/auth/logout`, () => HttpResponse.error()))
    const { router, queryClient } = renderApp(['/settings'])
    await screen.findByRole('heading', { name: 'Ajustes' })
    await userEvent.click(screen.getByRole('button', { name: 'Cerrar sesión' }))
    expect(
      await screen.findByText(
        'No pudimos cerrar la sesión. Revisá tu conexión.',
      ),
    ).toBeVisible()
    expect(router.state.location.pathname).toBe('/settings')
    expect(queryClient.getQueryData(['session'])).toEqual(session)
  })

  // T-F303: logout communicates and disables its action while the request is pending.
  it('disables the logout action while the request is pending', async () => {
    let release!: () => void
    const waiting = new Promise<void>((resolve) => {
      release = resolve
    })
    server.use(
      http.post(`${base}/auth/logout`, async () => {
        await waiting
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const user = userEvent.setup()
    renderApp(['/settings'])
    await screen.findByRole('heading', { name: 'Ajustes' })
    await user.click(screen.getByRole('button', { name: 'Cerrar sesión' }))
    expect(
      await screen.findByRole('button', { name: 'Cerrando sesión…' }),
    ).toBeDisabled()
    release()
    expect(await screen.findByText('Cerraste sesión.')).toBeVisible()
  })
})
