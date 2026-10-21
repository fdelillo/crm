import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { LoginPage } from './LoginPage'
import { buildSessionInfo, buildTenant, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { queryKeys } from '@/api/queryKeys'
import { ResetPasswordPage } from './ResetPasswordPage'

const base = 'https://localhost:5173/api/v1'

describe('ResetPasswordPage', () => {
  // T-F403 (US-2.3, BR-F06, DD-F16): confirms a new password with the hidden link token and clears the cached session.
  it('sends the fragment token only in the request body and replaces with the changed-password login', async () => {
    sessionStorage.setItem('crm.verifyBanner.dismissed', 'true')
    const request = vi.fn(async ({ request: req }: { request: Request }) => {
      expect(new URL(req.url).hash).toBe('')
      expect(new URL(req.url).search).toBe('')
      expect(await req.json()).toEqual({
        token: 'abc-_123',
        password: 'nueva-clave-segura',
      })
      return new HttpResponse(null, { status: 204 })
    })
    server.use(
      http.post(`${base}/auth/password-reset/confirm`, request),
      http.get(`${base}/me`, () =>
        HttpResponse.json(problem('unauthenticated', 401), {
          status: 401,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    const { router, queryClient } = renderApp(
      ['/reset-password#token=abc-_123'],
      {
        routes: [
          { path: '/reset-password', Component: ResetPasswordPage },
          { path: '/login', Component: LoginPage },
        ],
      },
    )
    queryClient.setQueryData(queryKeys.session, buildSessionInfo())
    const user = userEvent.setup()
    await user.type(
      await screen.findByLabelText('Nueva contraseña'),
      'nueva-clave-segura',
    )
    expect(screen.getByLabelText('Nueva contraseña')).toHaveAttribute(
      'autocomplete',
      'new-password',
    )
    await user.click(screen.getByRole('button', { name: 'Guardar contraseña' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(sessionStorage.getItem('crm.verifyBanner.dismissed')).toBeNull()
    expect(router.state.location.search).toBe('?reason=password_changed')
    expect(router.state.historyAction).toBe('REPLACE')
    expect(queryClient.getQueryData(queryKeys.session)).toBeUndefined()
    expect(
      await screen.findByRole('heading', { name: 'Ingresá a tu cuenta' }),
    ).toBeVisible()
    expect(
      screen.getByText('Listo, cambiaste tu contraseña. Ingresá con la nueva.'),
    ).toBeVisible()
    expect(request).toHaveBeenCalledOnce()
  })

  it('does not show the verification notice after reset and login with a verified session', async () => {
    let authenticated = false
    server.use(
      http.get(`${base}/me`, () =>
        authenticated
          ? HttpResponse.json(
              buildSessionInfo({
                user: { ...buildSessionInfo().user, email_verified: true },
              }),
            )
          : HttpResponse.json(problem('unauthenticated', 401), {
              status: 401,
              headers: { 'Content-Type': 'application/problem+json' },
            }),
      ),
      http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
      http.post(
        `${base}/auth/password-reset/confirm`,
        () => new HttpResponse(null, { status: 204 }),
      ),
      http.post(`${base}/auth/login`, () => {
        authenticated = true
        return HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        )
      }),
    )
    renderApp(['/reset-password#token=abc-_123'])
    const user = userEvent.setup()
    await user.type(
      await screen.findByLabelText('Nueva contraseña'),
      'nueva-clave-segura',
    )
    await user.click(screen.getByRole('button', { name: 'Guardar contraseña' }))
    await screen.findByText(
      'Listo, cambiaste tu contraseña. Ingresá con la nueva.',
    )
    await user.type(
      await screen.findByLabelText('Email'),
      'persona@example.test',
    )
    await user.type(screen.getByLabelText('Contraseña'), 'nueva-clave-segura')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    await screen.findByRole('heading', { name: 'Hola, Persona de prueba' })
    expect(screen.queryByText(/Confirmá tu email:/)).not.toBeInTheDocument()
  })

  it('shows the incomplete-link exit', async () => {
    renderApp(['/reset-password'], {
      routes: [{ path: '/reset-password', Component: ResetPasswordPage }],
    })
    expect(screen.getByText('Este enlace está incompleto.')).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Pedir un enlace nuevo' }),
    ).toHaveAttribute('href', '/forgot-password')
  })

  it('validates a short password without a request and exposes show/hide state', async () => {
    const request = vi.fn()
    server.use(http.post(`${base}/auth/password-reset/confirm`, request))
    renderApp(['/reset-password#token=abc-_123'], {
      routes: [{ path: '/reset-password', Component: ResetPasswordPage }],
    })
    const user = userEvent.setup()
    await user.type(
      await screen.findByLabelText('Nueva contraseña'),
      'shortpass',
    )
    await user.click(screen.getByRole('button', { name: 'Guardar contraseña' }))
    expect(
      await screen.findByText(
        'La contraseña tiene que tener al menos 10 caracteres.',
      ),
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Mostrar' })).toHaveAttribute(
      'aria-pressed',
      'false',
    )
    await user.click(screen.getByRole('button', { name: 'Mostrar' }))
    expect(screen.getByRole('button', { name: 'Ocultar' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    expect(request).not.toHaveBeenCalled()
  })

  it('shows a generic invalid-token explanation and a fresh-link action', async () => {
    server.use(
      http.post(`${base}/auth/password-reset/confirm`, () =>
        HttpResponse.json(problem('token_invalid', 400), {
          status: 400,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    renderApp(['/reset-password#token=abc-_123'], {
      routes: [{ path: '/reset-password', Component: ResetPasswordPage }],
    })
    const user = userEvent.setup()
    await user.type(
      await screen.findByLabelText('Nueva contraseña'),
      'nueva-clave-segura',
    )
    await user.click(screen.getByRole('button', { name: 'Guardar contraseña' }))
    expect(await screen.findByText('Este enlace ya no sirve.')).toBeVisible()
    expect(
      screen.getByText('Vence a la hora o ya se usó. Pedí uno nuevo.'),
    ).toBeVisible()
    expect(screen.queryByLabelText('Nueva contraseña')).not.toBeInTheDocument()
    const freshLink = screen.getByRole('link', {
      name: 'Pedir un enlace nuevo',
    })
    expect(freshLink).toHaveAttribute('href', '/forgot-password')
    await waitFor(() =>
      expect(
        screen.getByRole('heading', { name: 'Creá una contraseña nueva' }),
      ).toHaveFocus(),
    )
  })

  // T-F403 (DD-F16, §12.3): server field codes are mapped to user-facing text.
  it('shows the password message for a same-as-email validation error', async () => {
    server.use(
      http.post(`${base}/auth/password-reset/confirm`, () =>
        HttpResponse.json(
          problem('validation_failed', 422, {
            errors: [{ field: 'password', code: 'same_as_email' }],
          }),
          {
            status: 422,
            headers: { 'Content-Type': 'application/problem+json' },
          },
        ),
      ),
    )
    renderApp(['/reset-password#token=abc-_123'], {
      routes: [{ path: '/reset-password', Component: ResetPasswordPage }],
    })
    const user = userEvent.setup()
    await user.type(
      await screen.findByLabelText('Nueva contraseña'),
      'ana@example.test',
    )
    await user.click(screen.getByRole('button', { name: 'Guardar contraseña' }))
    expect(
      await screen.findByText('La contraseña no puede ser igual a tu email.'),
    ).toBeVisible()
  })
})
