import { act, screen, waitFor } from '@testing-library/react'
import { Outlet } from 'react-router'
import userEvent from '@testing-library/user-event'
import { onlineManager } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { queryKeys } from '@/api/queryKeys'
import {
  buildCurrentUser,
  buildSessionInfo,
  buildTenant,
  problem,
} from '@/test/fixtures'
import { gate } from '@/test/gate'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { RequireSession } from './routeGuards'

const base = 'https://localhost:5173/api/v1'
const forbiddenTitle = 'No tenés acceso a esta sección'
const operator = buildSessionInfo({
  user: buildCurrentUser({ role: 'operator', email_verified: true }),
  permissions: [],
})
function failure(status: 401 | 503) {
  return HttpResponse.json(
    problem(status === 401 ? 'unauthenticated' : 'service_unavailable', status),
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}
async function startSession(
  options: {
    strictMode?: boolean
    admin?: boolean
    onUnauthenticated?: () => void
  } = {},
) {
  server.use(http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())))
  const app = renderApp(['/'], {
    strictMode: options.strictMode,
    onUnauthenticated: options.onUnauthenticated,
    configureQueryClient: (client) => {
      client.setQueryDefaults(queryKeys.session, { gcTime: Infinity })
      client.setQueryDefaults(queryKeys.tenant, { gcTime: Infinity })
      client.setQueryData(
        queryKeys.session,
        options.admin ? buildSessionInfo() : operator,
      )
      client.setQueryData(queryKeys.tenant, buildTenant())
    },
  })
  await screen.findByRole('heading', { name: 'Hola, Persona de prueba' })
  return app
}
function expectChecking() {
  expect(
    screen.getByRole('status', { name: 'Comprobando tu acceso' }),
  ).toHaveAttribute('aria-busy', 'true')
  expect(
    screen.queryByRole('heading', { name: forbiddenTitle }),
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: 'Usuarios' }),
  ).not.toBeInTheDocument()
}
async function expectForbidden() {
  const heading = await screen.findByRole('heading', {
    level: 1,
    name: forbiddenTitle,
  })
  await waitFor(() => expect(heading).toHaveFocus())
  expect(document.title).toBe(`${forbiddenTitle} · Empresa de prueba`)
}
async function expectExpired(app: Awaited<ReturnType<typeof startSession>>) {
  await screen.findByRole('heading', { name: 'Ingresá a tu cuenta' })
  expect(
    app.router.state.location.pathname + app.router.state.location.search,
  ).toBe('/login?next=%2Fsettings%2Fusers&reason=session_expired')
  expect(
    screen.getByText('Tu sesión se cerró. Ingresá de nuevo para seguir.'),
  ).toBeVisible()
  expect(
    screen.queryByRole('heading', { name: forbiddenTitle }),
  ).not.toBeInTheDocument()
}

// T-F707 (DD-F34, INV-F21): real routes/cache; only the HTTP boundary is intercepted.
describe('permission checks on route entry', () => {
  beforeAll(async () => {
    await Promise.all([
      import('@/features/auth/pages'),
      import('@/features/users/pages'),
      import('@/features/dashboard/DashboardPage'),
      import('@/features/tenant/CompanyPage'),
      import('@/app/AppShell'),
    ])
  })
  afterEach(() => onlineManager.setOnline(true))

  it('opens login with the expired-session reason when entry revalidation returns 401', async () => {
    const app = await startSession()
    const me = vi.fn(() => failure(401))
    const users = vi.fn(() => HttpResponse.json({ items: [] }))
    server.use(http.get(`${base}/me`, me), http.get(`${base}/users`, users))
    await act(() => app.router.navigate('/settings/users'))
    await expectExpired(app)
    expect(users).not.toHaveBeenCalled()
  })
  it('shows only the access indicator until a retained 401 completes', async () => {
    const app = await startSession()
    const response = gate()
    const me = vi.fn(async () => {
      await response.opened
      return failure(401)
    })
    const users = vi.fn(() => HttpResponse.json({ items: [] }))
    server.use(http.get(`${base}/me`, me), http.get(`${base}/users`, users))
    try {
      await act(() => app.router.navigate('/settings/users'))
      expectChecking()
    } finally {
      response.open()
    }
    await expectExpired(app)
    expect(users).not.toHaveBeenCalled()
  })
  it('presents a focused forbidden heading after one successful denied-session read', async () => {
    const app = await startSession()
    const me = vi.fn(() => HttpResponse.json(operator))
    server.use(http.get(`${base}/me`, me))
    await act(() => app.router.navigate('/settings/users'))
    await expectForbidden()
    expect(me).toHaveBeenCalledOnce()
  })
  it('opens the requested page after promotion without ever presenting forbidden', async () => {
    const app = await startSession()
    const seenHeadings: string[] = []
    const observer = new MutationObserver((records) => {
      for (const record of records)
        for (const node of Array.from(record.addedNodes)) {
          if (!(node instanceof Element)) continue
          const headings = node.matches('h1')
            ? [node]
            : Array.from(node.querySelectorAll('h1'))
          seenHeadings.push(
            ...headings.map((heading) => heading.textContent ?? ''),
          )
        }
    })
    observer.observe(document.body, { childList: true, subtree: true })
    const response = gate()
    const me = vi.fn(async () => {
      await response.opened
      return HttpResponse.json(buildSessionInfo())
    })
    server.use(http.get(`${base}/me`, me))
    try {
      await act(() => app.router.navigate('/settings/users'))
      expectChecking()
      response.open()
      expect(
        await screen.findByRole('heading', { level: 1, name: 'Usuarios' }),
      ).toBeVisible()
      expect(seenHeadings).not.toContain(forbiddenTitle)
      expect(me).toHaveBeenCalledOnce()
    } finally {
      response.open()
      observer.disconnect()
    }
  })
  it.each([
    [
      'service unavailable',
      () => failure(503),
      'El servicio no está disponible en este momento.',
    ],
    ['network failure', () => HttpResponse.error(), 'No hay conexión.'],
  ] as const)(
    'presents a recoverable access error on %s',
    async (_name, fail, message) => {
      const app = await startSession()
      const me = vi.fn(fail)
      server.use(http.get(`${base}/me`, me))
      await act(() => app.router.navigate('/settings/users'))
      expect(
        await screen.findByRole('heading', {
          level: 1,
          name: 'No pudimos comprobar tu acceso',
        }),
      ).toBeVisible()
      expect(screen.getByText(message)).toBeVisible()
      expect(screen.getByRole('button', { name: 'Reintentar' })).toBeEnabled()
      expect(
        screen.getByRole('link', { name: 'Volver al inicio' }),
      ).toHaveAttribute('href', '/')
      expect(
        screen.queryByRole('heading', { name: forbiddenTitle }),
      ).not.toBeInTheDocument()
      expect(me).toHaveBeenCalledOnce()
    },
  )
  it.each(['click', 'keyboard'] as const)(
    'retries the access check by %s and focuses its result',
    async (input) => {
      const app = await startSession()
      const response = gate()
      const me = vi
        .fn()
        .mockImplementationOnce(() => failure(503))
        .mockImplementation(async () => {
          await response.opened
          return HttpResponse.json(operator)
        })
      server.use(http.get(`${base}/me`, me))
      await act(() => app.router.navigate('/settings/users'))
      const retry = await screen.findByRole('button', { name: 'Reintentar' })
      const user = userEvent.setup()
      try {
        if (input === 'click') await user.click(retry)
        else {
          const heading = screen.getByRole('heading', {
            name: 'No pudimos comprobar tu acceso',
          })
          await waitFor(() => expect(heading).toHaveFocus())
          await user.tab()
          expect(retry).toHaveFocus()
          await user.keyboard('{Enter}')
        }
        expectChecking()
      } finally {
        response.open()
      }
      await expectForbidden()
      expect(me).toHaveBeenCalledTimes(2)
    },
  )
  it('keeps the completed entry when unrelated tenant data rerenders the shell', async () => {
    const app = await startSession()
    const unexpectedRead = gate()
    const me = vi
      .fn()
      .mockImplementationOnce(() => HttpResponse.json(operator))
      .mockImplementation(async () => {
        await unexpectedRead.opened
        return HttpResponse.json(operator)
      })
    server.use(http.get(`${base}/me`, me))
    try {
      await act(() => app.router.navigate('/settings/users'))
      await expectForbidden()
      await act(async () => {
        app.queryClient.setQueryData(
          queryKeys.tenant,
          buildTenant({ name: 'Otra empresa de prueba' }),
        )
      })
      expect(
        await screen.findByText('Otra empresa de prueba', { exact: true }),
      ).toBeVisible()
      await expectForbidden()
      expect(me).toHaveBeenCalledOnce()
    } finally {
      unexpectedRead.open()
    }
  })
  it('checks each pathname entry when moving between denied sections and back', async () => {
    const app = await startSession()
    const me = vi.fn(() => HttpResponse.json(operator))
    server.use(http.get(`${base}/me`, me))
    for (const [index, path] of [
      '/settings/users',
      '/settings/company',
      '/settings/users',
    ].entries()) {
      await act(() => app.router.navigate(path))
      await expectForbidden()
      expect(me).toHaveBeenCalledTimes(index + 1)
    }
  })
  it('joins an in-flight session request instead of cancelling and replacing it', async () => {
    const app = await startSession()
    const response = gate()
    const me = vi.fn(async () => {
      await response.opened
      return HttpResponse.json(operator)
    })
    server.use(http.get(`${base}/me`, me))
    let refresh!: Promise<void>
    try {
      act(() => {
        refresh = app.queryClient.invalidateQueries({
          queryKey: queryKeys.session,
        })
      })
      await waitFor(() => expect(me).toHaveBeenCalledOnce())
      await act(() => app.router.navigate('/settings/users'))
      expectChecking()
      expect(me).toHaveBeenCalledOnce()
    } finally {
      response.open()
    }
    await act(() => refresh)
    await expectForbidden()
    expect(me).toHaveBeenCalledOnce()
  })
  it('sends one entry request under StrictMode', async () => {
    const app = await startSession({ strictMode: true })
    const response = gate()
    const me = vi.fn(async () => {
      await response.opened
      return HttpResponse.json(operator)
    })
    server.use(http.get(`${base}/me`, me))
    try {
      await act(() => app.router.navigate('/settings/users'))
      expectChecking()
      await waitFor(() => expect(me).toHaveBeenCalledOnce())
    } finally {
      response.open()
    }
    await expectForbidden()
    expect(me).toHaveBeenCalledOnce()
  })
  it('keeps the indicator offline and sends the check once connectivity returns', async () => {
    const app = await startSession()
    const me = vi.fn(() => HttpResponse.json(operator))
    server.use(http.get(`${base}/me`, me))
    act(() => onlineManager.setOnline(false))
    await act(() => app.router.navigate('/settings/users'))
    expectChecking()
    expect(me).not.toHaveBeenCalled()
    await act(async () => onlineManager.setOnline(true))
    await expectForbidden()
    expect(me).toHaveBeenCalledOnce()
  })
  it('opens a permitted page within staleTime without rereading the session', async () => {
    const app = await startSession({ admin: true })
    const me = vi.fn(() => HttpResponse.json(buildSessionInfo()))
    server.use(http.get(`${base}/me`, me))
    await act(() => app.router.navigate('/settings/users'))
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Usuarios' }),
    ).toBeVisible()
    expect(me).not.toHaveBeenCalled()
  })
})

const expiredUrl = '/login?next=%2Fsettings%2Fusers&reason=session_expired'
const expiredLine = 'Tu sesión se cerró. Ingresá de nuevo para seguir.'
const where = (app: Awaited<ReturnType<typeof startSession>>) =>
  app.router.state.location.pathname + app.router.state.location.search
const loader = () =>
  screen.queryByRole('status', { name: 'Cargando la aplicación' })

// Records every navigation the app asks the router for. With `hold`, the ones
// to /login wait before the router starts them, so a test can look at the
// screen while a session exit is under way.
function watchNavigation(
  app: Awaited<ReturnType<typeof startSession>>,
  hold?: Promise<void>,
) {
  const requested: string[] = []
  const original = app.router.navigate.bind(app.router) as (
    to: unknown,
    options?: unknown,
  ) => Promise<void>
  vi.spyOn(app.router, 'navigate').mockImplementation((async (
    to: unknown,
    options?: unknown,
  ) => {
    if (typeof to === 'string') requested.push(to)
    if (hold && typeof to === 'string' && to.startsWith('/login')) await hold
    return original(to, options)
  }) as typeof app.router.navigate)
  return { toLogin: () => requested.filter((to) => to.startsWith('/login')) }
}
async function losingTheSession(app: Awaited<ReturnType<typeof startSession>>) {
  const me = vi.fn(() => failure(401))
  server.use(http.get(`${base}/me`, me))
  await act(() =>
    app.queryClient.invalidateQueries({ queryKey: queryKeys.session }),
  )
  return me
}
// T-F707 (DD-F35, INV-F22, S-F16, ui.md §6.4.2): one session exit; the guard decides with the
// exit registry and never waits for a navigation nobody is making.
describe('leaving the session', () => {
  beforeAll(async () => {
    await Promise.all([
      import('@/features/auth/pages'),
      import('@/features/users/pages'),
      import('@/features/dashboard/DashboardPage'),
      import('@/features/tenant/CompanyPage'),
      import('@/app/AppShell'),
      import('@/app/SettingsPage'),
    ])
  })

  it('G1: shows the loader and asks once for the login while the exit is held', async () => {
    const app = await startSession({ admin: true })
    await act(() => app.router.navigate('/settings/users'))
    await screen.findByRole('heading', { level: 1, name: 'Usuarios' })
    const release = gate()
    const navigation = watchNavigation(app, release.opened)
    try {
      const me = await losingTheSession(app)
      await waitFor(() => expect(navigation.toLogin()).toHaveLength(1))
      expect(loader()).toBeVisible()
      expect(where(app)).toBe('/settings/users')
      expect(navigation.toLogin()).toEqual([expiredUrl])
      // Nothing reads the session again while the exit is under way.
      expect(me).toHaveBeenCalledOnce()
    } finally {
      release.open()
    }
    await expectExpired(app)
    expect(navigation.toLogin()).toEqual([expiredUrl])
  })

  it('G2: resumes an exit that another navigation interrupted, and never paints the page in between', async () => {
    const loginChunk = gate()
    const { LoginPage } = await import('@/features/auth/LoginPage')
    const app = renderApp(['/other', '/settings/users'], {
      routes: [
        {
          Component: Outlet,
          children: [
            {
              Component: RequireSession,
              children: [
                { path: 'other', element: <h1>Otra</h1> },
                { path: 'settings/users', element: <h1>Usuarios</h1> },
              ],
            },
            {
              path: 'login',
              lazy: async () => {
                await loginChunk.opened
                return { Component: LoginPage }
              },
            },
          ],
        },
      ],
      configureQueryClient: (client) => {
        client.setQueryDefaults(queryKeys.session, { gcTime: Infinity })
        client.setQueryData(queryKeys.session, buildSessionInfo())
      },
    })
    await screen.findByRole('heading', { name: 'Usuarios' })
    const shown: string[] = []
    const observer = new MutationObserver(() => {
      if (screen.queryByRole('heading', { name: 'Otra' })) shown.push('Otra')
    })
    observer.observe(document.body, { childList: true, subtree: true })
    const navigation = watchNavigation(app)
    try {
      await losingTheSession(app)
      await waitFor(() =>
        expect(app.router.state.navigation.location?.pathname).toBe('/login'),
      )
      await act(() => app.router.navigate(-1))
      await waitFor(() => expect(navigation.toLogin()).toHaveLength(2))
      expect(loader()).toBeVisible()
      expect(shown).toEqual([])
      expect(navigation.toLogin()).toEqual([expiredUrl, expiredUrl])
    } finally {
      loginChunk.open()
      observer.disconnect()
    }
    await screen.findByRole('heading', { name: 'Ingresá a tu cuenta' })
    expect(where(app)).toBe(expiredUrl)
    expect(loader()).toBeNull()
  })

  it('G3: leaves with the expired reason when the cache is cleared by someone else', async () => {
    const app = await startSession({ admin: true })
    await act(() => app.router.navigate('/settings'))
    await screen.findByRole('heading', { name: 'Ajustes' })
    server.use(http.get(`${base}/me`, () => failure(401)))
    act(() => app.queryClient.clear())
    await act(() => app.router.navigate('/'))
    await screen.findByRole('heading', { name: 'Ingresá a tu cuenta' })
    expect(where(app)).toBe('/login?next=%2F&reason=session_expired')
    expect(screen.getByText(expiredLine)).toBeVisible()
    expect(loader()).toBeNull()
  })

  it('G4: the guard leaves by itself when the global handler does not act', async () => {
    const app = await startSession({
      admin: true,
      onUnauthenticated: vi.fn(),
    })
    await act(() => app.router.navigate('/settings'))
    await screen.findByRole('heading', { name: 'Ajustes' })
    server.use(http.get(`${base}/me`, () => failure(401)))
    act(() => app.queryClient.clear())
    await act(() => app.router.navigate('/'))
    await screen.findByRole('heading', { name: 'Ingresá a tu cuenta' })
    expect(where(app)).toBe('/login?next=%2F&reason=session_expired')
    expect(screen.getByText(expiredLine)).toBeVisible()
    expect(loader()).toBeNull()
  })

  it('G5: going Back from the login to a protected page asks for the login without a reason', async () => {
    const app = await startSession({ admin: true })
    await act(() => app.router.navigate('/settings/users'))
    await screen.findByRole('heading', { level: 1, name: 'Usuarios' })
    await losingTheSession(app)
    await expectExpired(app)
    await act(() => app.router.navigate(-1))
    await waitFor(() => expect(where(app)).toBe('/login?next=%2F'))
    await screen.findByRole('heading', { name: 'Ingresá a tu cuenta' })
    expect(screen.queryByText(expiredLine)).toBeNull()
    expect(loader()).toBeNull()
  })

  it('G6: logging out keeps the logged-out reason while the exit is held and a 401 is waiting', async () => {
    const app = await startSession({ admin: true })
    await act(() => app.router.navigate('/settings'))
    await screen.findByRole('heading', { name: 'Ajustes' })
    const release = gate()
    const navigation = watchNavigation(app, release.opened)
    const logout = vi.fn(() => new HttpResponse(null, { status: 204 }))
    server.use(
      http.post(`${base}/auth/logout`, logout),
      http.get(`${base}/me`, () => failure(401)),
    )
    try {
      await userEvent.click(
        screen.getByRole('button', { name: 'Cerrar sesión' }),
      )
      await waitFor(() => expect(navigation.toLogin()).toHaveLength(1))
      expect(loader()).toBeVisible()
    } finally {
      release.open()
    }
    expect(await screen.findByText('Cerraste sesión.')).toBeVisible()
    expect(where(app)).toBe('/login?reason=logged_out')
    expect(logout).toHaveBeenCalledOnce()
    expect(navigation.toLogin()).toEqual(['/login?reason=logged_out'])
  })
})
