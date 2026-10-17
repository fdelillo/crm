import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { buildSessionInfo, problem } from '@/test/fixtures'
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
    server.use(
      http.post(`${base}/auth/login`, () => HttpResponse.json(validSession)),
    )
    const { router } = renderApp([entry])
    const user = await fillCredentials()
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(destination),
    )
    expect(router.state.historyAction).toBe('REPLACE')
  })

  // T-F301: invalid credentials keep email, clear/focus password and leave session untouched.
  it('shows one neutral credential message and focuses the emptied password', async () => {
    loggedOut()
    const login = vi.fn(() => serverProblem('invalid_credentials', 401))
    server.use(http.post(`${base}/auth/login`, login))
    const { router, queryClient } = renderApp(['/login'])
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
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 9, 17, 14, 20, 0))
    loggedOut()
    server.use(
      http.post(`${base}/auth/login`, () =>
        serverProblem('login_locked', 429, { 'Retry-After': '900' }),
      ),
    )
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    renderApp(['/login'])
    await screen.findByRole('heading', { name: 'Ingresar' })
    await user.type(screen.getByLabelText('Email'), 'ana@example.test')
    await user.type(screen.getByLabelText('Contraseña'), 'clave-segura')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(
      await screen.findByRole('heading', {
        name: 'Por seguridad, pausamos el ingreso con este email.',
      }),
    ).toBeVisible()
    expect(screen.getByText(/a las 14:35 \(en 15 minutos\)/)).toBeVisible()
    await act(async () => {
      vi.setSystemTime(new Date(2026, 9, 17, 14, 27, 0))
    })
    expect(screen.getByText(/a las 14:35 \(en 15 minutos\)/)).toBeVisible()
  })

  // T-F301 (P-F17): lock without Retry-After keeps both explicit exits available.
  it('shows a lock without a retry header and allows reset or retry', async () => {
    loggedOut()
    server.use(
      http.post(`${base}/auth/login`, () => serverProblem('login_locked', 429)),
    )
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

  // T-F301: login inputs use password-manager autocomplete and Enter submits.
  it('supports autocomplete and submitting with Enter from password', async () => {
    loggedOut()
    const login = vi.fn(() => HttpResponse.json(validSession))
    server.use(http.post(`${base}/auth/login`, login))
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
})
