import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { queryKeys } from '@/api/queryKeys'
import { buildSessionInfo, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { AcceptInvitationPage } from './pages'

const base = 'https://localhost:5173/api/v1'
const token = 'safe-fragment-token_123'
const preview = {
  tenant_name: 'Aberturas Norte',
  email: 'ana@example.test',
  role: 'operator' as const,
  expires_at: '2026-10-27T12:00:00Z',
}
const routes = [
  { path: '/accept-invitation', Component: AcceptInvitationPage },
  { path: '/login', element: <h1>Ingresá a tu cuenta</h1> },
  { path: '/forgot-password', element: <h1>Recuperar contraseña</h1> },
  { path: '/', element: <h1>Panel inicial</h1> },
]

function renderAccept(
  url = `/accept-invitation#token=${token}`,
  session: ReturnType<typeof buildSessionInfo> | null = null,
) {
  const app = renderApp([url], {
    routes,
    configureQueryClient: (client) =>
      client.setQueryData(queryKeys.session, session),
  })
  return app
}

describe('AcceptInvitationPage', () => {
  it('explains when the link has no token and does not call the preview endpoint', () => {
    const previewRequest = vi.fn()
    server.use(http.post(`${base}/auth/invitations/preview`, previewRequest))
    renderAccept('/accept-invitation')
    expect(
      screen.getByRole('heading', { name: 'Aceptar invitación' }),
    ).toBeVisible()
    expect(screen.getByText('Este enlace está incompleto.')).toBeVisible()
    expect(screen.getByText('Abrilo de nuevo desde el email.')).toBeVisible()
    expect(previewRequest).not.toHaveBeenCalled()
  })

  it('previews with the fragment token in the body only and shows read only invitation details', async () => {
    const request = vi.fn(async ({ request: req }: { request: Request }) => {
      const url = new URL(req.url)
      expect(url.hash).toBe('')
      expect(url.search).toBe('')
      expect(await req.json()).toEqual({ token })
      return HttpResponse.json(preview)
    })
    server.use(http.post(`${base}/auth/invitations/preview`, request))
    const { router } = renderAccept()
    expect(
      await screen.findByText('Te invitaron a Aberturas Norte como Operador'),
    ).toBeVisible()
    expect(screen.getByLabelText('Email')).toHaveValue('ana@example.test')
    expect(screen.getByLabelText('Email')).toHaveAttribute('readonly')
    await waitFor(() => expect(router.state.location.hash).toBe(''))
    expect(router.state.location.search).toBe('')
    expect(request).toHaveBeenCalledOnce()
  })

  it('shows a session conflict notice when another account is signed in', async () => {
    server.use(
      http.post(`${base}/auth/invitations/preview`, () =>
        HttpResponse.json(preview),
      ),
    )
    renderAccept(undefined, buildSessionInfo())
    expect(
      await screen.findByText(/Tenés una sesión abierta como/),
    ).toBeVisible()
  })

  it('blocks a password matching the invitation email without sending acceptance', async () => {
    const accept = vi.fn()
    server.use(
      http.post(`${base}/auth/invitations/preview`, () =>
        HttpResponse.json(preview),
      ),
      http.post(`${base}/auth/invitations/accept`, accept),
    )
    renderAccept()
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Tu nombre'), 'Ana Pérez')
    await user.type(screen.getByLabelText('Contraseña'), 'ana@example.test')
    await user.click(screen.getByRole('button', { name: 'Aceptar invitación' }))
    expect(
      await screen.findByText('La contraseña no puede ser igual a tu email.'),
    ).toBeVisible()
    expect(accept).not.toHaveBeenCalled()
  })

  it('maps server validation errors to name and password fields', async () => {
    server.use(
      http.post(`${base}/auth/invitations/preview`, () =>
        HttpResponse.json(preview),
      ),
      http.post(`${base}/auth/invitations/accept`, () =>
        HttpResponse.json(
          problem('validation_failed', 422, {
            errors: [
              { field: 'name', code: 'invalid_value' },
              { field: 'password', code: 'too_short' },
            ],
          }),
          {
            status: 422,
            headers: { 'Content-Type': 'application/problem+json' },
          },
        ),
      ),
    )
    renderAccept()
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Tu nombre'), 'Ana Pérez')
    await user.type(screen.getByLabelText('Contraseña'), 'muy-segura-123')
    await user.click(screen.getByRole('button', { name: 'Aceptar invitación' }))
    expect(
      await screen.findByText('Tiene que tener entre 1 y 120 caracteres.'),
    ).toBeVisible()
    expect(
      await screen.findByText(
        'La contraseña tiene que tener al menos 10 caracteres.',
      ),
    ).toBeVisible()
  })

  it('shows the neutral invalid-invitation state for a preview token_invalid response', async () => {
    server.use(
      http.post(`${base}/auth/invitations/preview`, () =>
        HttpResponse.json(problem('token_invalid', 400), {
          status: 400,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    renderAccept()
    expect(
      await screen.findByText('Esta invitación ya no es válida.'),
    ).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Pedir una nueva con tu email' }),
    ).toHaveAttribute('href', '/forgot-password')
    expect(screen.getByRole('link', { name: 'Ingresá' })).toHaveAttribute(
      'href',
      '/login',
    )
    expect(screen.queryByLabelText('Tu nombre')).not.toBeInTheDocument()
  })

  it('shows the invalid-invitation state if the token expires after preview', async () => {
    server.use(
      http.post(`${base}/auth/invitations/preview`, () =>
        HttpResponse.json(preview),
      ),
      http.post(`${base}/auth/invitations/accept`, () =>
        HttpResponse.json(problem('token_invalid', 400), {
          status: 400,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    renderAccept()
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Tu nombre'), 'Ana Pérez')
    await user.type(screen.getByLabelText('Contraseña'), 'clave-segura-123')
    await user.click(screen.getByRole('button', { name: 'Aceptar invitación' }))
    expect(
      await screen.findByText('Esta invitación ya no es válida.'),
    ).toBeVisible()
    expect(screen.queryByLabelText('Tu nombre')).not.toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Pedir una nueva con tu email' }),
    ).toHaveAttribute('href', '/forgot-password')
  })

  it('hides the form for a preview rate limit and offers retry', async () => {
    let attempts = 0
    server.use(
      http.post(`${base}/auth/invitations/preview`, () => {
        attempts += 1
        return attempts === 1
          ? HttpResponse.json(problem('rate_limited', 429), {
              status: 429,
              headers: {
                'Content-Type': 'application/problem+json',
                'Retry-After': '180',
              },
            })
          : HttpResponse.json(preview)
      }),
    )
    renderAccept()
    expect(
      await screen.findByText('Hiciste muchos intentos seguidos.'),
    ).toBeVisible()
    expect(screen.getByText(/Esperá 3 minutos y probá de nuevo/)).toBeVisible()
    expect(screen.queryByLabelText('Tu nombre')).not.toBeInTheDocument()
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(await screen.findByLabelText('Tu nombre')).toBeVisible()
    expect(attempts).toBe(2)
  })

  it('accepts the invitation, replaces with home, seeds the session and clears the old banner mark', async () => {
    const accepted = buildSessionInfo({
      tenant: { ...buildSessionInfo().tenant, name: 'Aberturas Norte' },
      user: {
        ...buildSessionInfo().user,
        name: 'Ana Pérez',
        email: preview.email,
        email_verified: false,
      },
    })
    const request = vi.fn(async ({ request: req }: { request: Request }) => {
      expect(new URL(req.url).hash).toBe('')
      expect(new URL(req.url).search).toBe('')
      expect(await req.json()).toEqual({
        token,
        name: 'Ana Pérez',
        password: 'clave-segura-123',
      })
      return HttpResponse.json(accepted, { status: 201 })
    })
    server.use(
      http.post(`${base}/auth/invitations/preview`, () =>
        HttpResponse.json(preview),
      ),
      http.post(`${base}/auth/invitations/accept`, request),
    )
    sessionStorage.setItem('crm.verifyBanner.dismissed', 'true')
    const { router, queryClient } = renderAccept()
    queryClient.setQueryData(queryKeys.tenant, { id: 'old-tenant' })
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Tu nombre'), 'Ana Pérez')
    await user.type(screen.getByLabelText('Contraseña'), 'clave-segura-123')
    await user.click(screen.getByRole('button', { name: 'Aceptar invitación' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(router.state.historyAction).toBe('REPLACE')
    expect(queryClient.getQueryData(queryKeys.session)).toEqual(accepted)
    expect(queryClient.getQueryData(queryKeys.tenant)).toBeUndefined()
    expect(sessionStorage.getItem('crm.verifyBanner.dismissed')).toBeNull()
    expect(
      await screen.findByRole('heading', { name: 'Panel inicial' }),
    ).toBeVisible()
    expect(
      await screen.findByText('¡Bienvenido/a a Aberturas Norte!'),
    ).toBeVisible()
    expect(request).toHaveBeenCalledOnce()
  })
})
