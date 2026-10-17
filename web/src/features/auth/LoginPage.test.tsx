import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { Application } from '@/app/application'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { buildSessionInfo, buildTenant, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'

const base = 'https://localhost:5173/api/v1'
const validSession = buildSessionInfo()
function serverProblem(
  code: Parameters<typeof problem>[0],
  status: number,
  headers?: Record<string, string>,
) {
  return HttpResponse.json(problem(code, status), {
    status,
    headers: { 'Content-Type': 'application/problem+json', ...headers },
  })
}
function loggedOut() {
  server.use(
    http.get(`${base}/me`, () => serverProblem('unauthenticated', 401)),
  )
}
async function fillCredentials() {
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('Email'), ' ana@example.test ')
  await user.type(screen.getByLabelText('Contraseña'), 'clave-segura')
  return user
}
afterEach(() => vi.useRealTimers())

describe('LoginPage', () => {
  // T-F301: successful login replaces history and accepts only safe next paths.
  it.each([
    ['/login', '/'],
    ['/login?next=%2Fsettings%2Fusers', '/settings/users'],
    ['/login?next=%2F%2Fevil.example', '/'],
  ])('replaces the login entry for %s with %s', async (entry, destination) => {
    loggedOut()
    let authenticated = false
    server.use(
      http.get(`${base}/me`, () =>
        authenticated
          ? HttpResponse.json(validSession)
          : serverProblem('unauthenticated', 401),
      ),
      http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
      http.get(`${base}/users`, () => HttpResponse.json({ items: [] })),
      http.post(`${base}/auth/login`, () => {
        authenticated = true
        return HttpResponse.json(validSession)
      }),
    )
    const { router, queryClient } = renderApp([entry])
    queryClient.setQueryData(['stale-session-data'], 'stale')
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(destination),
    )
    expect(router.state.historyAction).toBe('REPLACE')
    if (destination === '/') {
      expect(queryClient.getQueryData(['stale-session-data'])).toBeUndefined()
      expect(queryClient.getQueryData(['session'])).toEqual(validSession)
    }
    await act(async () => router.navigate(-1))
    expect(router.state.location.pathname).toBe(destination)
  })

  // T-F301: invalid credentials keep email, clear/focus password and leave session untouched.
  it('shows one neutral credential message and focuses the emptied password', async () => {
    loggedOut()
    const login = vi.fn(() => serverProblem('invalid_credentials', 401))
    server.use(http.post(`${base}/auth/login`, login))
    const { router, queryClient } = renderApp(['/login'])
    queryClient.setQueryDefaults(['unrelated-cache'], { gcTime: Infinity })
    queryClient.setQueryData(['unrelated-cache'], 'keep')
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'El email o la contraseña no son correctos.',
    )
    expect(screen.getByLabelText('Email')).toHaveValue('ana@example.test')
    expect(screen.getByLabelText('Contraseña')).toHaveValue('')
    expect(screen.getByLabelText('Contraseña')).toHaveFocus()
    expect(router.state.location.pathname).toBe('/login')
    expect(queryClient.getQueryData(['session'])).toBeNull()
    expect(queryClient.getQueryData(['unrelated-cache'])).toBe('keep')
    expect(login).toHaveBeenCalledOnce()
  })

  // T-F301: disabled accounts get the contract copy without a credential-specific recovery action.
  it('announces a disabled account with its administrator guidance', async () => {
    loggedOut()
    server.use(
      http.post(`${base}/auth/login`, () =>
        serverProblem('account_disabled', 403),
      ),
    )
    renderApp(['/login'])
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Tu usuario está desactivado.',
    )
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Pedile a un administrador de tu empresa que lo reactive.',
    )
  })

  // T-F301 (P-F18): lock time stays anchored to when Retry-After was received.
  it('shows the retry time derived from the received 900 seconds', async () => {
    const receivedAt = new Date(2026, 9, 17, 14, 20, 0).getTime()
    const now = vi.spyOn(Date, 'now').mockReturnValue(receivedAt)
    loggedOut()
    server.use(
      http.post(`${base}/auth/login`, () =>
        serverProblem('login_locked', 429, { 'Retry-After': '900' }),
      ),
    )
    const { router, queryClient, rerender } = renderApp(['/login'])
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Email'), 'ana@example.test')
    await user.type(screen.getByLabelText('Contraseña'), 'clave-segura')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(
      screen.getByRole('heading', {
        name: 'Por seguridad, pausamos el ingreso con este email.',
      }),
    ).toBeVisible()
    expect(screen.getByText(/a las 14:35 \(en 15 minutos\)/)).toBeVisible()
    act(() => {
      now.mockReturnValue(new Date(2026, 9, 17, 14, 27, 0).getTime())
      rerender(<Application router={router} queryClient={queryClient} />)
    })
    expect(screen.getByText(/a las 14:35 \(en 15 minutos\)/)).toBeVisible()
  })

  // T-F301 (P-F17): lock without Retry-After keeps both explicit exits available.
  it('shows a lock without a retry header and allows reset or retry', async () => {
    loggedOut()
    server.use(
      http.post(`${base}/auth/login`, () => serverProblem('login_locked', 429)),
    )
    const { router } = renderApp(['/login'])
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(
      await screen.findByText(/volver a intentar en unos minutos/),
    ).toBeVisible()
    await user.click(
      screen.getByRole('button', { name: 'Restablecer contraseña' }),
    )
    expect(
      screen.getByRole('heading', { name: 'Olvidé mi contraseña' }),
    ).toBeVisible()
    expect(router.state.location.state).toEqual({ email: 'ana@example.test' })
  })

  // T-F301 (DD-F17): the locked state can return to the preserved form.
  it('returns from the lock notice to the form without losing the email', async () => {
    loggedOut()
    server.use(
      http.post(`${base}/auth/login`, () => serverProblem('login_locked', 429)),
    )
    renderApp(['/login'])
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    await screen.findByRole('heading', {
      name: 'Por seguridad, pausamos el ingreso con este email.',
    })
    await user.click(screen.getByRole('button', { name: 'Volver a intentar' }))
    expect(screen.getByLabelText('Email')).toHaveValue('ana@example.test')
    expect(screen.getByLabelText('Contraseña')).toHaveValue('clave-segura')
  })

  // T-F301 (DD-F25): rate limit waits use the Retry-After range.
  it.each([
    ['3', 'Esperá unos segundos y probá de nuevo.'],
    ['120', 'Esperá 2 minutos y probá de nuevo.'],
  ])('explains rate limit Retry-After %s', async (seconds, copy) => {
    loggedOut()
    server.use(
      http.post(`${base}/auth/login`, () =>
        serverProblem('rate_limited', 429, { 'Retry-After': seconds }),
      ),
    )
    renderApp(['/login'])
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(copy)
  })

  // T-F301 (DD-F32): login notices depend on URL reason and lost-form state.
  it.each([
    [
      '/login?reason=session_expired',
      {},
      'Tu sesión se cerró. Ingresá de nuevo para seguir.',
    ],
    [
      '/login?reason=password_changed',
      {},
      'Listo, cambiaste tu contraseña. Ingresá con la nueva.',
    ],
    ['/login?reason=logged_out', {}, 'Cerraste sesión.'],
    [
      '/login?reason=logged_out',
      { unsavedChangesLost: true },
      'Cerraste sesión.',
    ],
    ['/login?reason=other', {}, null],
    [
      '/login?reason=session_expired',
      { unsavedChangesLost: true },
      'Los cambios que no habías guardado se perdieron.',
    ],
  ])('shows the correct notice for %s', async (path, state, copy) => {
    loggedOut()
    const location = new URL(path, 'https://localhost')
    renderApp([{ pathname: location.pathname, search: location.search, state }])
    if (copy) expect(await screen.findByText(copy)).toBeVisible()
    else expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  // T-F301: forgot-password carries the typed email in navigation state.
  it('passes the entered email to password recovery', async () => {
    loggedOut()
    const { router } = renderApp(['/login'])
    const user = await fillCredentials()
    await user.click(
      screen.getByRole('link', { name: '¿Olvidaste tu contraseña?' }),
    )
    expect(router.state.location.pathname).toBe('/forgot-password')
    expect(router.state.location.state).toEqual({ email: 'ana@example.test' })
  })

  // T-F301: retryable failures preserve credentials and submit only after an explicit retry.
  it('preserves values after 503 and retries without navigation', async () => {
    loggedOut()
    let recovered = false
    const login = vi.fn(() =>
      recovered
        ? HttpResponse.json(validSession)
        : serverProblem('service_unavailable', 503),
    )
    server.use(http.post(`${base}/auth/login`, login))
    const { router } = renderApp(['/login'])
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'El servicio no está disponible en este momento.',
    )
    expect(screen.getByLabelText('Email')).toHaveValue('ana@example.test')
    expect(screen.getByLabelText('Contraseña')).toHaveValue('clave-segura')
    recovered = true
    await user.click(screen.getByRole('button', { name: 'Reintentar' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(login).toHaveBeenCalledTimes(2)
  })

  // T-F301: offline failures use the network message, preserve data, and retry on request.
  it('preserves credentials after a network failure and retries explicitly', async () => {
    loggedOut()
    let recovered = false
    const login = vi.fn(() =>
      recovered ? HttpResponse.json(validSession) : HttpResponse.error(),
    )
    server.use(http.post(`${base}/auth/login`, login))
    const { router } = renderApp(['/login'])
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'No hay conexión.',
    )
    expect(screen.getByLabelText('Email')).toHaveValue('ana@example.test')
    expect(screen.getByLabelText('Contraseña')).toHaveValue('clave-segura')
    recovered = true
    await user.click(screen.getByRole('button', { name: 'Reintentar' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(login).toHaveBeenCalledTimes(2)
  })

  // T-F301: login inputs use password-manager autocomplete and Enter submits.
  it('supports autocomplete and submitting with Enter from password', async () => {
    loggedOut()
    const login = vi.fn(() => HttpResponse.json(validSession))
    server.use(
      http.post(`${base}/auth/login`, login),
      http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
    )
    renderApp(['/login'])
    const user = await fillCredentials()
    expect(screen.getByLabelText('Email')).toHaveAttribute(
      'autocomplete',
      'email',
    )
    expect(screen.getByLabelText('Contraseña')).toHaveAttribute(
      'autocomplete',
      'current-password',
    )
    await user.keyboard('{Enter}')
    await waitFor(() => expect(login).toHaveBeenCalledOnce())
  })

  // T-F302: client validation uses the contract's field error codes before sending.
  it('validates email format locally and focuses the invalid field', async () => {
    loggedOut()
    const login = vi.fn(() => HttpResponse.json(validSession))
    server.use(http.post(`${base}/auth/login`, login))
    renderApp(['/login'])
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Email'), 'not-an-email')
    await user.type(screen.getByLabelText('Contraseña'), 'clave-segura')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(
      await screen.findByText(
        'Ingresá un email válido, por ejemplo nombre@empresa.com',
      ),
    ).toBeVisible()
    expect(screen.getByLabelText('Email')).toHaveFocus()
    expect(login).not.toHaveBeenCalled()
  })

  // T-F302: 422 field errors use the shared server-error mapping.
  it('maps validation field errors to their login field and message', async () => {
    loggedOut()
    server.use(
      http.post(`${base}/auth/login`, () =>
        HttpResponse.json(
          problem('validation_failed', 422, {
            errors: [{ field: 'email', code: 'invalid_format' }],
          }),
          {
            status: 422,
            headers: { 'Content-Type': 'application/problem+json' },
          },
        ),
      ),
    )
    renderApp(['/login'])
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(
      await screen.findByText(
        'Ingresá un email válido, por ejemplo nombre@empresa.com',
      ),
    ).toBeVisible()
    expect(screen.getByLabelText('Email')).toHaveFocus()
  })

  // T-F302: the pending guard ignores rapid duplicate submits while the request is held.
  it('sends only one login when the user submits again while pending', async () => {
    loggedOut()
    let release!: () => void
    const waiting = new Promise<void>((resolve) => {
      release = resolve
    })
    const login = vi.fn(async () => {
      await waiting
      return HttpResponse.json(validSession)
    })
    server.use(http.post(`${base}/auth/login`, login))
    renderApp(['/login'])
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    await screen.findByRole('button', { name: 'Ingresando…' })
    await user.click(screen.getByRole('button', { name: 'Ingresando…' }))
    expect(login).toHaveBeenCalledOnce()
    release()
  })
})
