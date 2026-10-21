import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
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
    sessionStorage.setItem('crm.verifyBanner.dismissed', 'true')
    let loggedOut = false
    let sessionReads = 0
    let logoutCompleted = false
    let tenantReadsAfterLogout = 0
    server.use(
      http.get(`${base}/me`, () => {
        sessionReads += 1
        return loggedOut ? unauthenticated() : HttpResponse.json(session)
      }),
      http.get(`${base}/tenant`, () => {
        if (logoutCompleted) tenantReadsAfterLogout += 1
        return logoutCompleted
          ? unauthenticated()
          : HttpResponse.json(buildTenant())
      }),
      http.post(`${base}/auth/logout`, () => {
        loggedOut = true
        logoutCompleted = true
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const { router, queryClient } = renderApp(['/', '/settings'])
    await screen.findByRole('heading', { name: 'Ajustes' })
    await waitFor(() =>
      expect(queryClient.getQueryData(['tenant'])).toBeDefined(),
    )
    let cacheWasEmptyAtLoggedOutLocation = false
    const unsubscribe = router.subscribe((state) => {
      if (
        state.location.pathname === '/login' &&
        state.location.search === '?reason=logged_out'
      ) {
        cacheWasEmptyAtLoggedOutLocation =
          queryClient.getQueryCache().getAll().length === 0
      }
    })
    await userEvent.click(screen.getByRole('button', { name: 'Cerrar sesión' }))
    expect(await screen.findByText('Cerraste sesión.')).toBeVisible()
    expect(router.state.location.pathname).toBe('/login')
    expect(router.state.location.search).toBe('?reason=logged_out')
    expect(router.state.historyAction).toBe('REPLACE')
    expect(queryClient.getQueryData(['tenant'])).toBeUndefined()
    expect(sessionStorage.getItem('crm.verifyBanner.dismissed')).toBeNull()
    expect(cacheWasEmptyAtLoggedOutLocation).toBe(true)

    const readsBeforeBack = sessionReads
    await router.navigate(-1)
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(sessionReads).toBe(readsBeforeBack)
    expect(
      screen.queryByRole('heading', { name: 'Ajustes' }),
    ).not.toBeInTheDocument()
    expect(screen.queryByText('Empresa de prueba')).not.toBeInTheDocument()
    expect(tenantReadsAfterLogout).toBe(0)
    unsubscribe()
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
