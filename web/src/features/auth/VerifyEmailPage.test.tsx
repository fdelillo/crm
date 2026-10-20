import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { buildSessionInfo, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { VerifyEmailPage } from './VerifyEmailPage'

const base = 'https://localhost:5173/api/v1'
const serverProblem = (
  code: Parameters<typeof problem>[0],
  status: number,
  headers: Record<string, string> = {},
) =>
  HttpResponse.json(problem(code, status), {
    status,
    headers: { 'Content-Type': 'application/problem+json', ...headers },
  })

describe('VerifyEmailPage', () => {
  // T-F404 (US-1.3, DD-13, DD-F3, P-F10): confirmation is an explicit user action with the account-safety warning.
  it('does not confirm on page load and confirms only after the user presses the button', async () => {
    const confirm = vi.fn(() => new HttpResponse(null, { status: 204 }))
    server.use(http.post(`${base}/auth/email-verification/confirm`, confirm))
    server.use(
      http.get(`${base}/me`, () => serverProblem('unauthenticated', 401)),
    )
    renderApp(['/verify-email#token=verify-_123'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    expect(
      await screen.findByText(/Si no creaste una cuenta con este email/),
    ).toBeVisible()
    expect(confirm).not.toHaveBeenCalled()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Confirmar mi email' }))
    expect(
      await screen.findByText('¡Listo! Tu email quedó confirmado.'),
    ).toBeVisible()
    expect(screen.getByRole('link', { name: 'Ingresar' })).toBeVisible()
    expect(confirm).toHaveBeenCalledOnce()
  })

  it('offers resend only for an unverified signed-in session after an invalid link', async () => {
    const me = vi.fn(() =>
      HttpResponse.json(
        buildSessionInfo({
          user: { ...buildSessionInfo().user, email_verified: false },
        }),
      ),
    )
    server.use(
      http.get(`${base}/me`, me),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        serverProblem('token_invalid', 400),
      ),
    )
    renderApp(['/verify-email#token=bad-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    expect(
      await screen.findByText('Este enlace de confirmación ya no sirve.'),
    ).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Reenviar email de confirmación' }),
    ).toBeVisible()
    expect(me).toHaveBeenCalled()
  })

  it('shows the confirmed-session state without a resend action when token is missing', async () => {
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
      ),
    )
    renderApp(['/verify-email'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    await waitFor(() =>
      expect(
        screen.getByText(
          'Tu email ya está confirmado. No hace falta otro enlace.',
        ),
      ).toBeVisible(),
    )
    expect(
      screen.queryByRole('button', { name: 'Reenviar email de confirmación' }),
    ).not.toBeInTheDocument()
  })

  it('does not offer resend for an invalid token when the session is already verified', async () => {
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
      ),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        serverProblem('token_invalid', 400),
      ),
    )
    renderApp(['/verify-email#token=expired-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    await waitFor(() =>
      expect(
        screen.getByRole('heading', { name: 'Confirmá tu email' }),
      ).toBeVisible(),
    )
    expect(
      screen.queryByRole('button', { name: 'Reenviar email de confirmación' }),
    ).not.toBeInTheDocument()
  })

  // T-F406 (DD-F31, INV-F17): the shared resend hook must bypass cached session data.
  it('uses the follow-up /me result after resend and never reports sent when it says verified', async () => {
    let getCalls = 0
    server.use(
      http.get(`${base}/me`, () => {
        getCalls += 1
        return HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: getCalls > 1 },
          }),
        )
      }),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        serverProblem('token_invalid', 400),
      ),
      http.post(
        `${base}/auth/email-verification/resend`,
        () => new HttpResponse(null, { status: 202 }),
      ),
    )
    renderApp(['/verify-email#token=expired-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    await user.click(
      await screen.findByRole('button', {
        name: 'Reenviar email de confirmación',
      }),
    )
    await waitFor(() =>
      expect(
        screen.getAllByText(
          'Tu email ya está confirmado. No hace falta otro enlace.',
        ).length,
      ).toBeGreaterThanOrEqual(1),
    )
    expect(
      screen.queryByText(/Listo, te lo reenviamos/),
    ).not.toBeInTheDocument()
    expect(getCalls).toBe(2)
  })

  // T-F404 (DD-46, DD-F31): a resend result must not become a confirmation error.
  it('shows only the resend success after an invalid link when /me stays unverified', async () => {
    let sessionReads = 0
    const resend = vi.fn(() => new HttpResponse(null, { status: 202 }))
    server.use(
      http.get(`${base}/me`, () => {
        sessionReads += 1
        return HttpResponse.json(buildSessionInfo())
      }),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        serverProblem('token_invalid', 400),
      ),
      http.post(`${base}/auth/email-verification/resend`, resend),
    )
    renderApp(['/verify-email#token=bad-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    await user.click(
      await screen.findByRole('button', {
        name: 'Reenviar email de confirmación',
      }),
    )
    expect(
      await screen.findByText(
        'Listo, te lo reenviamos. Revisá también el correo no deseado.',
      ),
    ).toBeVisible()
    expect(screen.getAllByRole('alert')).toHaveLength(1)
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Este enlace de confirmación ya no sirve.',
    )
    expect(screen.queryByText('Algo salió mal de nuestro lado.')).toBeNull()
    expect(sessionReads).toBe(2)
    expect(resend).toHaveBeenCalledOnce()
  })

  // T-F404 (DD-F29): resend 429 uses the resend-specific title and Retry-After.
  it('shows the resend-specific rate-limit message after an invalid link', async () => {
    server.use(
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        serverProblem('token_invalid', 400),
      ),
      http.post(`${base}/auth/email-verification/resend`, () =>
        serverProblem('rate_limited', 429, { 'Retry-After': '120' }),
      ),
    )
    renderApp(['/verify-email#token=bad-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    await user.click(
      await screen.findByRole('button', {
        name: 'Reenviar email de confirmación',
      }),
    )
    expect(
      await screen.findByText('Ya te lo mandamos hace poco.'),
    ).toBeVisible()
    expect(screen.getByText('Esperá 2 minutos y probá de nuevo.')).toBeVisible()
    expect(
      screen.queryByText('Hiciste muchos intentos seguidos.'),
    ).not.toBeInTheDocument()
    expect(screen.getAllByRole('alert')).toHaveLength(2)
  })

  // T-F404 (DD-13): confirm success with a session remains account-neutral.
  it('refreshes the signed-in session after confirmation and returns to the home page', async () => {
    let meReads = 0
    server.use(
      http.get(`${base}/me`, () => {
        meReads += 1
        return HttpResponse.json(buildSessionInfo())
      }),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        new HttpResponse(null, { status: 204 }),
      ),
    )
    const { router } = renderApp(['/verify-email#token=valid-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    expect(
      await screen.findByText('¡Listo! Tu email quedó confirmado.'),
    ).toBeVisible()
    expect(screen.getByRole('link', { name: 'Ir al inicio' })).toHaveAttribute(
      'href',
      '/',
    )
    expect(meReads).toBeGreaterThanOrEqual(2)
    expect(screen.queryByText(/invitar/)).not.toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/verify-email')
  })

  // T-F404 (DD-F29): confirm errors render once and use the confirmation context.
  it('shows one confirmation alert for a rate limit', async () => {
    server.use(
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        serverProblem('rate_limited', 429, { 'Retry-After': '120' }),
      ),
    )
    renderApp(['/verify-email#token=valid-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    expect(
      await screen.findByText('Hiciste muchos intentos seguidos.'),
    ).toBeVisible()
    expect(screen.getAllByRole('alert')).toHaveLength(1)
    expect(screen.queryByText('Ya te lo mandamos hace poco.')).toBeNull()
  })

  // T-F404 (DD-13): retrying confirmation must never trigger the resend endpoint.
  it('offers one confirmation retry and does not resend when confirmation fails offline', async () => {
    const resend = vi.fn()
    server.use(
      http.get(`${base}/me`, () => serverProblem('unauthenticated', 401)),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        serverProblem('service_unavailable', 503),
      ),
      http.post(`${base}/auth/email-verification/resend`, resend),
    )
    renderApp(['/verify-email#token=valid-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    const retry = await screen.findByRole('button', { name: 'Reintentar' })
    expect(screen.getAllByRole('alert')).toHaveLength(1)
    expect(screen.getAllByRole('button', { name: 'Reintentar' })).toHaveLength(
      1,
    )
    await user.click(retry)
    expect(resend).not.toHaveBeenCalled()
    expect(screen.getAllByRole('alert')).toHaveLength(1)
  })

  // T-F404 (DD-F31): opening a token while signed in must still allow its confirmation.
  it('keeps the confirmation action for a token even when the signed-in email is verified', async () => {
    const confirm = vi.fn(() => new HttpResponse(null, { status: 204 }))
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
      ),
      http.post(`${base}/auth/email-verification/confirm`, confirm),
    )
    renderApp(['/verify-email#token=other-account-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    expect(
      await screen.findByText(
        'Si no creaste una cuenta con este email, no lo confirmes: confirmarlo le daría acceso a quien la creó.',
      ),
    ).toBeVisible()
    const user = userEvent.setup()
    await user.click(
      screen.getByRole('button', { name: 'Confirmar mi email' }),
    )
    expect(
      await screen.findByText('¡Listo! Tu email quedó confirmado.'),
    ).toBeVisible()
    expect(confirm).toHaveBeenCalledOnce()
  })

  // T-F404 (DD-F31): success and already-verified outcomes focus the page heading.
  it('moves focus to the heading after confirmation succeeds', async () => {
    server.use(
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        new HttpResponse(null, { status: 204 }),
      ),
    )
    renderApp(['/verify-email#token=valid-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    const heading = await screen.findByRole('heading', {
      name: 'Confirmá tu email',
    })
    await waitFor(() => expect(heading).toHaveFocus())
  })

  // T-F404 (DD-F31): incomplete links explain the state and offer the correct exits.
  it('shows the incomplete-link state and login exit for an unverified visitor', async () => {
    server.use(
      http.get(`${base}/me`, () => serverProblem('unauthenticated', 401)),
    )
    renderApp(['/verify-email'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    expect(
      await screen.findByText('Este enlace está incompleto.'),
    ).toBeVisible()
    expect(screen.getByRole('link', { name: 'Ingresar' })).toHaveAttribute(
      'href',
      '/login',
    )
  })

  // T-F404 (DD-13): invalid tokens without a session must keep the login exit.
  it('shows the invalid-link message and login exit to a visitor', async () => {
    server.use(
      http.get(`${base}/me`, () => serverProblem('unauthenticated', 401)),
      http.post(`${base}/auth/email-verification/confirm`, () =>
        serverProblem('token_invalid', 400),
      ),
    )
    renderApp(['/verify-email#token=bad-token'], {
      routes: [{ path: '/verify-email', Component: VerifyEmailPage }],
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Confirmar mi email' }),
    )
    expect(
      await screen.findByText('Este enlace de confirmación ya no sirve.'),
    ).toBeVisible()
    expect(screen.getByRole('link', { name: 'Ingresar' })).toHaveAttribute(
      'href',
      '/login',
    )
  })
})
