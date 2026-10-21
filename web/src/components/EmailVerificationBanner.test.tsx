import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { buildSessionInfo, buildTenant, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { EmailVerificationBanner } from './EmailVerificationBanner'

const base = 'https://localhost:5173/api/v1'
const dismissalKey = 'crm.verifyBanner.dismissed'
function loggedIn(emailVerified = false) {
  server.use(
    http.get(`${base}/me`, () =>
      HttpResponse.json(
        buildSessionInfo({
          user: { ...buildSessionInfo().user, email_verified: emailVerified },
        }),
      ),
    ),
    http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
  )
}

describe('EmailVerificationBanner', () => {
  it('renders both parts of an error message passed as one value', () => {
    render(
      <EmailVerificationBanner
        email="persona@example.test"
        resendStatus="error"
        errorMessage={{
          title: 'Título del error',
          description: 'Detalle del error',
        }}
        onResend={() => undefined}
        onDismiss={() => undefined}
      />,
    )

    expect(screen.getByText('Título del error')).toBeVisible()
    expect(screen.getByText('Detalle del error')).toBeVisible()
  })

  it('does not show the notice for an already verified session', async () => {
    loggedIn(true)
    renderApp(['/settings/users'], { protectedPage: <h1>Usuarios</h1> })
    await screen.findByRole('heading', { name: 'Usuarios' })
    expect(screen.queryByText(/Confirmá tu email:/)).not.toBeInTheDocument()
  })

  // T-F405 (BR-F05, DD-46, DD-F31, INV-F17): the signed-in shell reflects the server session after a resend.
  it('shows the verification email, waits for resend, then announces success from a fresh /me read', async () => {
    const events: string[] = []
    server.use(
      http.get(`${base}/me`, () => {
        events.push('me')
        return HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: false },
          }),
        )
      }),
      http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
      http.post(`${base}/auth/email-verification/resend`, async () => {
        events.push('resend')
        await new Promise((resolve) => setTimeout(resolve, 50))
        return new HttpResponse(null, { status: 202 })
      }),
    )
    renderApp(['/settings/users'], {
      protectedPage: <h1 tabIndex={-1}>Usuarios</h1>,
    })
    expect(
      await screen.findByText(
        'Confirmá tu email: te mandamos un enlace a persona@example.test.',
      ),
    ).toBeVisible()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Reenviar' }))
    expect(screen.getByRole('button', { name: 'Enviando…' })).toBeDisabled()
    expect(
      await screen.findByText(
        'Listo, te lo reenviamos. Revisá también el correo no deseado.',
      ),
    ).toBeVisible()
    expect(
      screen.getByText(
        'Listo, te lo reenviamos. Revisá también el correo no deseado.',
      ),
    ).toHaveAttribute('aria-live', 'polite')
    expect(screen.getByRole('button', { name: 'Reenviar' })).toBeEnabled()
    expect(screen.queryByText(/20 minutos/)).not.toBeInTheDocument()
    expect(events).toEqual(['me', 'resend', 'me'])
  })

  it('recognizes an email confirmed in another window and removes the notice with focus on the page heading', async () => {
    let verified = false
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: verified },
          }),
        ),
      ),
      http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
      http.post(`${base}/auth/email-verification/resend`, () => {
        verified = true
        return new HttpResponse(null, { status: 202 })
      }),
    )
    renderApp(['/settings/users'], {
      protectedPage: <h1 tabIndex={-1}>Usuarios</h1>,
    })
    await screen.findByText(/Confirmá tu email:/)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Reenviar' }))
    await waitFor(() =>
      expect(screen.queryByText(/Confirmá tu email:/)).not.toBeInTheDocument(),
    )
    expect(
      await screen.findByText(
        'Tu email ya está confirmado. No hace falta otro enlace.',
      ),
    ).toBeVisible()
    expect(screen.getByRole('heading', { name: 'Usuarios' })).toHaveFocus()
    expect(
      screen.queryByText(/Listo, te lo reenviamos/),
    ).not.toBeInTheDocument()
  })

  // T-F405 (DD-F28, INV-F10): a background session refresh preserves active user focus.
  it('keeps focus in the active field when a session refresh learns the email is verified', async () => {
    let verified = false
    let sessionReads = 0
    server.use(
      http.get(`${base}/me`, () => {
        sessionReads += 1
        return HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: verified },
          }),
        )
      }),
      http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
    )
    const { queryClient } = renderApp(['/settings/users'], {
      protectedPage: (
        <>
          <h1 tabIndex={-1}>Usuarios</h1>
          <input aria-label="Buscar usuarios" />
        </>
      ),
    })
    const input = await screen.findByRole('textbox', {
      name: 'Buscar usuarios',
    })
    expect(screen.getByText(/Confirmá tu email:/)).toBeVisible()
    input.focus()
    verified = true
    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: ['session'],
        exact: true,
      })
    })
    await waitFor(() =>
      expect(screen.queryByText(/Confirmá tu email:/)).not.toBeInTheDocument(),
    )
    expect(sessionReads).toBe(2)
    expect(input).toHaveFocus()
  })

  it('keeps the notice and gives neutral text if the follow-up session read fails', async () => {
    loggedIn(false)
    let meCalls = 0
    server.use(
      http.get(`${base}/me`, () =>
        ++meCalls === 1
          ? HttpResponse.json(
              buildSessionInfo({
                user: { ...buildSessionInfo().user, email_verified: false },
              }),
            )
          : HttpResponse.json(problem('internal', 500), {
              status: 500,
              headers: { 'Content-Type': 'application/problem+json' },
            }),
      ),
      http.post(
        `${base}/auth/email-verification/resend`,
        () => new HttpResponse(null, { status: 202 }),
      ),
    )
    renderApp(['/settings/users'], { protectedPage: <h1>Usuarios</h1> })
    await screen.findByText(/Confirmá tu email:/)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Reenviar' }))
    expect(
      await screen.findByText(
        /Si todavía no lo confirmaste, te mandamos un enlace nuevo/,
      ),
    ).toBeVisible()
    expect(screen.getByText(/Confirmá tu email:/)).toBeVisible()
    expect(
      screen.queryByText(/Listo, te lo reenviamos/),
    ).not.toBeInTheDocument()
  })

  it.each([
    ['300', '5 minutos'],
    ['1200', '20 minutos'],
  ])(
    'shows the exact Retry-After wait for %s seconds',
    async (seconds, wait) => {
      loggedIn(false)
      server.use(
        http.post(`${base}/auth/email-verification/resend`, () =>
          HttpResponse.json(problem('rate_limited', 429), {
            status: 429,
            headers: {
              'Content-Type': 'application/problem+json',
              'Retry-After': seconds,
            },
          }),
        ),
      )
      renderApp(['/settings/users'], { protectedPage: <h1>Usuarios</h1> })
      await screen.findByText(/Confirmá tu email:/)
      const user = userEvent.setup()
      await user.click(screen.getByRole('button', { name: 'Reenviar' }))
      expect(
        await screen.findByText('Ya te lo mandamos hace poco.'),
      ).toBeVisible()
      expect(
        await screen.findByText(`Esperá ${wait} y probá de nuevo.`),
      ).toBeVisible()
      expect(screen.getByRole('button', { name: 'Reenviar' })).toBeEnabled()
    },
  )

  it('updates both rate-limit texts when an IP limit is followed by the user limit', async () => {
    loggedIn(false)
    let requests = 0
    server.use(
      http.post(`${base}/auth/email-verification/resend`, () => {
        requests += 1
        const seconds = requests === 1 ? '180' : '1200'
        return HttpResponse.json(problem('rate_limited', 429), {
          status: 429,
          headers: {
            'Content-Type': 'application/problem+json',
            'Retry-After': seconds,
          },
        })
      }),
    )
    renderApp(['/settings/users'], { protectedPage: <h1>Usuarios</h1> })
    await screen.findByText(/Confirmá tu email:/)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Reenviar' }))
    expect(
      await screen.findByText('Ya te lo mandamos hace poco.'),
    ).toBeVisible()
    expect(
      await screen.findByText('Esperá 3 minutos y probá de nuevo.'),
    ).toBeVisible()

    await user.click(screen.getByRole('button', { name: 'Reenviar' }))
    expect(
      await screen.findByText('Ya te lo mandamos hace poco.'),
    ).toBeVisible()
    expect(
      await screen.findByText('Esperá 20 minutos y probá de nuevo.'),
    ).toBeVisible()
    expect(requests).toBe(2)
  })

  it('shows both title and description for other resend errors', async () => {
    loggedIn(false)
    server.use(
      http.post(`${base}/auth/email-verification/resend`, () =>
        HttpResponse.json(problem('service_unavailable', 503), {
          status: 503,
          headers: {
            'Content-Type': 'application/problem+json',
            'Retry-After': '60',
          },
        }),
      ),
    )
    renderApp(['/settings/users'], { protectedPage: <h1>Usuarios</h1> })
    await screen.findByText(/Confirmá tu email:/)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Reenviar' }))
    expect(
      await screen.findByText(
        'El servicio no está disponible en este momento.',
      ),
    ).toBeVisible()
    expect(await screen.findByText('Probá de nuevo en 1 minuto.')).toBeVisible()
  })

  it('exposes named keyboard controls in resend then dismiss order', async () => {
    loggedIn(false)
    renderApp(['/settings/users'], { protectedPage: <h1>Usuarios</h1> })
    await screen.findByText(/Confirmá tu email:/)
    const resend = screen.getByRole('button', { name: 'Reenviar' })
    const dismiss = screen.getByRole('button', {
      name: 'Ocultar aviso de confirmación de email',
    })
    resend.focus()
    await userEvent.setup().tab()
    expect(dismiss).toHaveFocus()
  })

  it('keeps a dismissed notice hidden across navigation and restores it after session storage is cleared', async () => {
    sessionStorage.clear()
    loggedIn(false)
    const { router, unmount } = renderApp(['/settings/users'], {
      protectedPage: <h1>Usuarios</h1>,
    })
    await screen.findByText(/Confirmá tu email:/)
    const user = userEvent.setup()
    await user.click(
      screen.getByRole('button', {
        name: 'Ocultar aviso de confirmación de email',
      }),
    )
    expect(sessionStorage.getItem(dismissalKey)).toBe('true')
    expect(
      [...Array(sessionStorage.length)].map((_, index) =>
        sessionStorage.key(index),
      ),
    ).toEqual([dismissalKey])
    await waitFor(() =>
      expect(screen.queryByText(/Confirmá tu email:/)).not.toBeInTheDocument(),
    )
    expect(
      screen.queryByRole('button', { name: 'Reenviar' }),
    ).not.toBeInTheDocument()
    await act(async () => router.navigate('/settings'))
    expect(screen.queryByText(/Confirmá tu email:/)).not.toBeInTheDocument()

    unmount()
    renderApp(['/settings/users'], {
      protectedPage: <h1>Usuarios</h1>,
    })
    expect(screen.queryByText(/Confirmá tu email:/)).not.toBeInTheDocument()

    sessionStorage.clear()
    cleanup()
    renderApp(['/settings/users'], {
      protectedPage: <h1>Usuarios</h1>,
    })
    expect(await screen.findByText(/Confirmá tu email:/)).toBeVisible()
  })
})
