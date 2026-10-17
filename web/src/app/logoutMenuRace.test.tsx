import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { buildSessionInfo, buildTenant, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'

vi.mock('@/features/auth/pages', async (importOriginal) => {
  await new Promise((resolve) => setTimeout(resolve, 300))
  return importOriginal()
})

const base = 'https://localhost:5173/api/v1'
const session = buildSessionInfo()
const unauthenticated = () =>
  HttpResponse.json(problem('unauthenticated', 401), {
    status: 401,
    headers: { 'Content-Type': 'application/problem+json' },
  })

describe('logout menu navigation race', () => {
  let loggedOut = false
  let logoutCompleted = false
  let tenantReadsAfterLogout = 0
  let tenant401sAfterLogout = 0
  let releaseTenant401!: () => void
  let navigationPending: Promise<void>
  let unsubscribeRouter: (() => void) | undefined

  beforeEach(() => {
    loggedOut = false
    logoutCompleted = false
    tenantReadsAfterLogout = 0
    tenant401sAfterLogout = 0
    navigationPending = new Promise<void>((resolve) => {
      releaseTenant401 = resolve
    })
    server.use(
      http.get(`${base}/me`, () =>
        loggedOut ? unauthenticated() : HttpResponse.json(session),
      ),
      http.get(`${base}/tenant`, async () => {
        if (!logoutCompleted) return HttpResponse.json(buildTenant())
        tenantReadsAfterLogout += 1
        await navigationPending
        tenant401sAfterLogout += 1
        return unauthenticated()
      }),
      http.post(`${base}/auth/logout`, () => {
        loggedOut = true
        logoutCompleted = true
        return new HttpResponse(null, { status: 204 })
      }),
    )
  })

  afterEach(() => {
    unsubscribeRouter?.()
    releaseTenant401()
    server.resetHandlers()
  })

  // Review 2: keep the login module lazy while a post-logout tenant 401 can race its navigation.
  it('preserves the logged-out reason and never starts a tenant read after logout', async () => {
    const { router } = renderApp(['/'])
    const user = userEvent.setup()
    await screen.findByText('Rubro: Genérico')

    unsubscribeRouter = router.subscribe((state) => {
      if (state.navigation.location?.pathname === '/login') {
        releaseTenant401()
      }
    })
    const menu = await screen.findByRole('button', {
      name: /Persona de prueba · Administrador/,
    })
    await user.click(menu)
    await user.click(
      await screen.findByRole('menuitem', { name: 'Cerrar sesión' }),
    )

    await waitFor(() =>
      expect(
        router.state.location.pathname + router.state.location.search,
      ).toBe('/login?reason=logged_out'),
    )
    expect(await screen.findByText('Cerraste sesión.')).toBeVisible()
    await new Promise((resolve) => setTimeout(resolve, 350))
    expect(tenantReadsAfterLogout).toBe(0)
    expect(tenant401sAfterLogout).toBe(0)
  })
})
