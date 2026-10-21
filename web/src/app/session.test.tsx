import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { focusManager, useMutation, useQuery } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { unwrap } from '@/api/errors'
import { queryKeys } from '@/api/queryKeys'
import {
  buildSessionInfo,
  buildCurrentUser,
  buildTenant,
  problem,
} from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'

const base = 'https://localhost:5173/api/v1'
function denied(code: Parameters<typeof problem>[0], status: number) {
  return HttpResponse.json(problem(code, status), {
    status,
    headers: { 'Content-Type': 'application/problem+json' },
  })
}
function sessionHandlers(session = buildSessionInfo()) {
  server.use(
    http.get(`${base}/me`, () => HttpResponse.json(session)),
    http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
  )
}
function Probe({ mutate = false }: { mutate?: boolean }) {
  const query = useQuery({
    queryKey: ['protected-probe'],
    queryFn: async () => unwrap(await api.GET('/users')),
    enabled: !mutate,
  })
  const mutation = useMutation({
    mutationFn: async () =>
      unwrap(
        await api.POST('/auth/login', {
          body: { email: 'test@example.test', password: 'wrong' },
        }),
      ),
  })
  return (
    <>
      <h1>Acción protegida</h1>
      <button onClick={() => mutation.mutate()}>Ejecutar acción</button>
      {query.isError && <p>Error de consulta</p>}
      {mutation.isError && <p>Error de acción</p>}
    </>
  )
}
// T-F104 (INV-F01/02/03/06/16): real router/cache and HTTP interception at the boundary.
describe('session guards and global failures', () => {
  beforeAll(async () => {
    await Promise.all([
      import('@/features/auth/pages'),
      import('@/app/AppShell'),
    ])
  })
  afterEach(() => focusManager.setFocused(undefined))
  it('waits for the session without flashing login', async () => {
    server.use(http.get(`${base}/me`, async () => new Promise(() => {})))
    renderApp(['/settings/users'])
    expect(
      await screen.findByRole('status', { name: 'Cargando la aplicación' }),
    ).toHaveAttribute('aria-busy', 'true')
    expect(
      screen.queryByRole('heading', { name: 'Ingresá a tu cuenta' }),
    ).not.toBeInTheDocument()
  })
  it('redirects an anonymous visitor without an expiration notice', async () => {
    server.use(http.get(`${base}/me`, () => denied('unauthenticated', 401)))
    const { router } = renderApp(['/settings/users'])
    expect(
      await screen.findByRole('heading', { name: 'Ingresá a tu cuenta' }),
    ).toBeVisible()
    expect(router.state.location.pathname + router.state.location.search).toBe(
      '/login?next=%2Fsettings%2Fusers',
    )
    expect(
      screen.queryByText('Tu sesión se cerró. Ingresá de nuevo para seguir.'),
    ).not.toBeInTheDocument()
  })
  it.each([
    ['/login', '/'],
    ['/login?next=/settings/users', '/settings/users'],
    ['/login?next=//evil.example', '/'],
  ])('redirects authenticated visitors at %s', async (path, target) => {
    sessionHandlers()
    const { router } = renderApp([path!])
    await waitFor(() => expect(router.state.location.pathname).toBe(target))
  })
  it('retries a failed session read', async () => {
    server.use(http.get(`${base}/me`, () => denied('service_unavailable', 503)))
    renderApp(['/settings/users'])
    await screen.findByRole('button', { name: 'Reintentar' })
    sessionHandlers()
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(
      await screen.findByRole('heading', { name: 'Usuarios' }),
    ).toBeVisible()
  })
  // T-F104/T-F106 (review: same-URL recovery): focus and title follow the presented screen.
  it('updates title and heading focus after exhausted session retries recover', async () => {
    const me = vi.fn(() => denied('service_unavailable', 503))
    server.use(http.get(`${base}/me`, me))
    renderApp(['/settings/users'], {
      configureQueryClient: (client) => {
        const defaults = client.getDefaultOptions()
        client.setDefaultOptions({
          ...defaults,
          queries: { ...defaults.queries, retry: 2, retryDelay: 0 },
        })
      },
    })
    await screen.findByRole('heading', {
      name: 'El servicio no está disponible en este momento.',
    })
    expect(me).toHaveBeenCalledTimes(3)
    expect(document.title).toBe(
      'El servicio no está disponible en este momento. · CRM',
    )
    sessionHandlers()
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    const heading = await screen.findByRole('heading', {
      name: 'Usuarios',
    })
    await waitFor(() =>
      expect(document.title).toBe('Usuarios · Empresa de prueba'),
    )
    await waitFor(() => expect(heading).toHaveFocus())
  })
  // T-F104 (P-F18, ui.md §6.4): a failed background refetch keeps the shell and shows no notice.
  it.each([
    ['a 503', () => denied('service_unavailable', 503)],
    ['a network error', () => HttpResponse.error()],
  ])(
    'keeps the shell mounted when a focus refetch of the cached session fails with %s',
    async (_name, failure) => {
      const now = Date.now()
      const clock = vi.spyOn(Date, 'now').mockReturnValue(now)
      sessionHandlers()
      const { queryClient } = renderApp(['/settings'], {
        configureQueryClient: (client) => {
          const defaults = client.getDefaultOptions()
          client.setDefaultOptions({
            ...defaults,
            queries: { ...defaults.queries, retry: 2, retryDelay: 0 },
          })
        },
      })
      const heading = await screen.findByRole('heading', { name: 'Ajustes' })
      heading.tabIndex = -1
      heading.focus()
      const nav = screen.getByRole('link', { name: 'Inicio' })
      const me = vi.fn(failure)
      server.use(http.get(`${base}/me`, me))
      clock.mockReturnValue(now + 61_000)
      await act(async () => {
        focusManager.setFocused(false)
        focusManager.setFocused(true)
      })
      await waitFor(() => expect(me).toHaveBeenCalledTimes(3))
      await waitFor(() =>
        expect(queryClient.getQueryState(queryKeys.session)?.status).toBe(
          'error',
        ),
      )
      expect(screen.getByRole('heading', { name: 'Ajustes' })).toBe(heading)
      expect(heading).toHaveFocus()
      expect(screen.getByRole('link', { name: 'Inicio' })).toBe(nav)
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(
        screen.queryByRole('button', { name: 'Reintentar' }),
      ).not.toBeInTheDocument()
      // The next refetch succeeds and nothing visible changes.
      sessionHandlers()
      clock.mockReturnValue(now + 130_000)
      await act(async () => {
        focusManager.setFocused(false)
        focusManager.setFocused(true)
      })
      await waitFor(() =>
        expect(queryClient.getQueryState(queryKeys.session)?.status).toBe(
          'success',
        ),
      )
      expect(screen.getByRole('heading', { name: 'Ajustes' })).toBe(heading)
      expect(heading).toHaveFocus()
    },
  )
  it('sends a 401 on first load to login', async () => {
    server.use(http.get(`${base}/me`, () => denied('unauthenticated', 401)))
    const { router } = renderApp(['/settings'])
    await screen.findByRole('heading', { name: 'Ingresá a tu cuenta' })
    expect(router.state.location.pathname).toBe('/login')
  })
  it('does not mount an unauthorized page or request its data', async () => {
    sessionHandlers(
      buildSessionInfo({
        user: buildCurrentUser({ role: 'operator' }),
        permissions: [],
      }),
    )
    const users = vi.fn(() => HttpResponse.json({ items: [] }))
    server.use(http.get(`${base}/users`, users))
    renderApp(['/settings/users'], { protectedPage: <Probe /> })
    expect(
      await screen.findByRole('heading', {
        name: 'No tenés acceso a esta sección',
      }),
    ).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Volver al inicio' }),
    ).toHaveAttribute('href', '/')
    expect(users).not.toHaveBeenCalled()
  })
  it('clears protected data before replacing the route after query expiration and rechecks on Back', async () => {
    sessionHandlers()
    server.use(http.get(`${base}/users`, () => denied('unauthenticated', 401)))
    const { router, queryClient } = renderApp(['/', '/settings/users'], {
      protectedPage: <Probe />,
    })
    queryClient.setQueryDefaults(['private-data'], { gcTime: Infinity })
    queryClient.setQueryData(['private-data'], { secret: 'fixture' })
    const clear = vi.spyOn(queryClient, 'clear')
    await screen.findByText('Tu sesión se cerró. Ingresá de nuevo para seguir.')
    expect(router.state.location.search).toBe(
      '?next=%2Fsettings%2Fusers&reason=session_expired',
    )
    expect(queryClient.getQueryData(['private-data'])).toBeUndefined()
    expect(clear).toHaveBeenCalledOnce()
    server.use(http.get(`${base}/me`, () => denied('unauthenticated', 401)))
    await waitFor(() =>
      expect(queryClient.getQueryData(queryKeys.session)).toBeNull(),
    )
    await act(() => router.navigate(-1))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(
      screen.queryByRole('heading', { name: 'Panel inicial' }),
    ).not.toBeInTheDocument()
  })
  it.each([
    ['unauthenticated', 401, true],
    ['invalid_credentials', 401, false],
    ['email_not_verified', 403, false],
  ] as const)(
    'handles mutation %s by stable code',
    async (code, status, expired) => {
      sessionHandlers()
      server.use(http.post(`${base}/auth/login`, () => denied(code, status)))
      const { router, queryClient } = renderApp(['/settings/users'], {
        protectedPage: <Probe mutate />,
      })
      await screen.findByRole('button', { name: 'Ejecutar acción' })
      queryClient.setQueryDefaults(['private-data'], { gcTime: Infinity })
      queryClient.setQueryData(['private-data'], 'fixture')
      await userEvent.click(
        screen.getByRole('button', { name: 'Ejecutar acción' }),
      )
      if (expired) {
        await screen.findByText(
          'Tu sesión se cerró. Ingresá de nuevo para seguir.',
        )
        expect(queryClient.getQueryData(['private-data'])).toBeUndefined()
      } else {
        await screen.findByText('Error de acción')
        expect(router.state.location.pathname).toBe('/settings/users')
        expect(queryClient.getQueryData(['private-data'])).toBe('fixture')
        expect(
          screen.queryByText('No tenés permiso para hacer esto.'),
        ).not.toBeInTheDocument()
      }
    },
  )
  it.each([false, true])(
    'refreshes permissions after forbidden (mutation %s)',
    async (mutate) => {
      sessionHandlers()
      const reject = () => {
        sessionHandlers(
          buildSessionInfo({
            permissions: [],
            user: buildCurrentUser({ role: 'operator' }),
          }),
        )
        return denied('forbidden', 403)
      }
      server.use(
        http.get(`${base}/users`, reject),
        http.post(`${base}/auth/login`, reject),
      )
      renderApp(['/settings/users'], {
        protectedPage: <Probe mutate={mutate} />,
      })
      if (mutate)
        await userEvent.click(
          await screen.findByRole('button', { name: 'Ejecutar acción' }),
        )
      expect(
        await screen.findByRole('heading', {
          name: 'No tenés acceso a esta sección',
        }),
      ).toBeVisible()
      if (mutate)
        expect(
          (await screen.findAllByText('No tenés permiso para hacer esto.'))[0],
        ).toBeVisible()
    },
  )
  it.each([false, true])(
    'uses verification-aware focus refresh (%s)',
    async (verified) => {
      const now = Date.now()
      const clock = vi.spyOn(Date, 'now').mockReturnValue(now)
      const me = vi.fn(() =>
        HttpResponse.json(
          buildSessionInfo({
            user: buildCurrentUser({ email_verified: verified }),
          }),
        ),
      )
      sessionHandlers()
      server.use(http.get(`${base}/me`, me))
      renderApp(['/settings'])
      await screen.findByRole('heading', { name: 'Ajustes' })
      clock.mockReturnValue(now + 10_000)
      await act(async () => {
        focusManager.setFocused(false)
        focusManager.setFocused(true)
      })
      if (!verified) await waitFor(() => expect(me).toHaveBeenCalledTimes(2))
      else expect(me).toHaveBeenCalledTimes(1)
    },
  )
  it('navigates only once when simultaneous queries lose the session', async () => {
    sessionHandlers()
    const { router, queryClient } = renderApp(['/settings/users'])
    await screen.findByRole('heading', { name: 'Usuarios' })
    server.use(http.get(`${base}/me`, () => denied('unauthenticated', 401)))
    const navigate = vi.spyOn(router, 'navigate')
    server.use(
      http.get(`${base}/users`, () => denied('unauthenticated', 401)),
      http.get(`${base}/tenant`, () => denied('unauthenticated', 401)),
    )
    await act(async () => {
      await Promise.allSettled([
        queryClient.fetchQuery({
          queryKey: ['concurrent-users'],
          queryFn: async () => unwrap(await api.GET('/users')),
        }),
        queryClient.fetchQuery({
          queryKey: ['concurrent-tenant'],
          queryFn: async () => unwrap(await api.GET('/tenant')),
        }),
      ])
    })
    await screen.findByText('Tu sesión se cerró. Ingresá de nuevo para seguir.')
    expect(
      navigate.mock.calls.filter(
        ([to]) =>
          typeof to === 'string' && to.includes('reason=session_expired'),
      ),
    ).toHaveLength(1)
  })
  it('treats session-to-null refresh as expiration', async () => {
    sessionHandlers()
    const { queryClient, router } = renderApp(['/settings/users'])
    await screen.findByRole('heading', { name: 'Usuarios' })
    server.use(http.get(`${base}/me`, () => denied('unauthenticated', 401)))
    await act(() =>
      queryClient.invalidateQueries({ queryKey: queryKeys.session }),
    )
    await screen.findByText('Tu sesión se cerró. Ingresá de nuevo para seguir.')
    expect(router.state.location.search).toContain('reason=session_expired')
  })
})
