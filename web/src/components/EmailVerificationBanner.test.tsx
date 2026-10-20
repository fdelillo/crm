import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { buildSessionInfo, buildTenant } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'

const base = 'https://localhost:5173/api/v1'
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
  // T-F405 (BR-F05, DD-46, DD-F31, INV-F17): the signed-in shell reflects the server session after a resend.
  it('shows the verification email, waits for resend, then announces success from a fresh /me read', async () => {
    loggedIn(false)
    let sends = 0
    server.use(
      http.post(`${base}/auth/email-verification/resend`, async () => {
        sends += 1
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
    expect(
      await screen.findByText(
        'Listo, te lo reenviamos. Revisá también el correo no deseado.',
      ),
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Reenviar' })).toBeEnabled()
    expect(screen.queryByText(/20 minutos/)).not.toBeInTheDocument()
    expect(sends).toBe(1)
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
          : HttpResponse.json(
              {
                type: '/problems/internal',
                title: 'err',
                status: 500,
                code: 'internal',
              },
              {
                status: 500,
                headers: { 'Content-Type': 'application/problem+json' },
              },
            ),
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
          HttpResponse.json(
            {
              type: '/problems/rate_limited',
              title: 'limit',
              status: 429,
              code: 'rate_limited',
            },
            {
              status: 429,
              headers: {
                'Content-Type': 'application/problem+json',
                'Retry-After': seconds,
              },
            },
          ),
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
        return HttpResponse.json(
          {
            type: '/problems/rate_limited',
            title: 'limit',
            status: 429,
            code: 'rate_limited',
          },
          {
            status: 429,
            headers: {
              'Content-Type': 'application/problem+json',
              'Retry-After': seconds,
            },
          },
        )
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
        HttpResponse.json(
          {
            type: '/problems/service_unavailable',
            title: 'unavailable',
            status: 503,
            code: 'service_unavailable',
          },
          {
            status: 503,
            headers: {
              'Content-Type': 'application/problem+json',
              'Retry-After': '60',
            },
          },
        ),
      ),
    )
    renderApp(['/settings/users'], { protectedPage: <h1>Usuarios</h1> })
    await screen.findByText(/Confirmá tu email:/)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Reenviar' }))
    expect(
      await screen.findByText('El servicio no está disponible en este momento.'),
    ).toBeVisible()
    expect(await screen.findByText('Probá de nuevo en 1 minuto.')).toBeVisible()
  })

  it('can hide the notice for the lifetime of the browser tab session', async () => {
    loggedIn(false)
    renderApp(['/settings/users'], { protectedPage: <h1>Usuarios</h1> })
    await screen.findByText(/Confirmá tu email:/)
    const user = userEvent.setup()
    await user.click(
      screen.getByRole('button', {
        name: 'Ocultar aviso de confirmación de email',
      }),
    )
    await waitFor(() =>
      expect(screen.queryByText(/Confirmá tu email:/)).not.toBeInTheDocument(),
    )
  })
})
