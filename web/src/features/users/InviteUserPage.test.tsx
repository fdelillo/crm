import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { InviteUserPage, UsersPage } from './pages'
import { buildSessionInfo, buildUser, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { queryKeys } from '@/api/queryKeys'
import { focusManager } from '@tanstack/react-query'

const base = 'https://localhost:5173/api/v1'
const routes = [
  { path: '/settings/users/invite', Component: InviteUserPage },
  { path: '/settings/users', element: <h1>Usuarios</h1> },
]

function renderInvite(
  emailVerified = true,
  options: Parameters<typeof renderApp>[1] = {},
) {
  return renderApp(['/settings/users/invite'], {
    ...options,
    routes: options.routes ?? routes,
    configureQueryClient: (client) => {
      client.setQueryData(
        queryKeys.session,
        buildSessionInfo({
          user: { ...buildSessionInfo().user, email_verified: emailVerified },
        }),
      )
      options.configureQueryClient?.(client)
    },
  })
}

describe('InviteUserPage', () => {
  afterEach(() => focusManager.setFocused(undefined))
  // T-F504 (US-3.1, BR-F08, BR-F11, BR-F14, DD-F6, DD-F21, DD-F24, DD-F29): invite form.
  it('starts with Operator selected and explains both roles', () => {
    renderInvite()
    expect(
      screen.getByRole('heading', { name: 'Invitar a alguien' }),
    ).toBeVisible()
    expect(screen.getByRole('radio', { name: /Operador/ })).toBeChecked()
    expect(
      screen.getByText(
        'Carga clientes, proyectos, presupuestos y cobros. No ve finanzas ni ajustes.',
      ),
    ).toBeVisible()
    expect(screen.getByText('Ve y gestiona todo.')).toBeVisible()
  })

  it('validates email before the request and returns to the list on cancel', async () => {
    const request = vi.fn()
    server.use(http.post(`${base}/users/invitations`, request))
    renderInvite()
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), 'invalido')
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(
      await screen.findByText(
        'Ingresá un email válido, por ejemplo nombre@empresa.com',
      ),
    ).toBeVisible()
    expect(request).not.toHaveBeenCalled()
    await user.click(screen.getByRole('link', { name: 'Cancelar' }))
    expect(
      await screen.findByRole('heading', { name: 'Usuarios' }),
    ).toBeVisible()
  })

  it('creates an invitation, returns to users and announces its expiry', async () => {
    const email = 'operador@example.test'
    let createdUser: ReturnType<typeof buildUser> | null = null
    const request = vi.fn(async ({ request: req }: { request: Request }) => {
      expect(await req.json()).toEqual({ email, role: 'operator' })
      createdUser = buildUser({
        id: 'invited',
        email,
        name: null,
        role: 'operator',
        status: 'invited',
        invitation_expires_at: '2026-10-27T12:00:00Z',
      })
      return HttpResponse.json(createdUser, { status: 201 })
    })
    server.use(
      http.post(`${base}/users/invitations`, request),
      http.get(`${base}/users`, () =>
        HttpResponse.json({
          items: [buildUser(), ...(createdUser ? [createdUser] : [])],
        }),
      ),
    )
    const { router } = renderInvite(true, {
      routes: [
        { path: '/settings/users/invite', Component: InviteUserPage },
        { path: '/settings/users', Component: UsersPage },
      ],
    })
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), email)
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/settings/users'),
    )
    expect(
      await screen.findByRole('heading', { name: 'Usuarios' }),
    ).toBeVisible()
    expect(
      await screen.findByText(
        'Invitación enviada a operador@example.test. Vence el 27/10, 09:00.',
      ),
    ).toBeVisible()
    expect(await screen.findAllByText('operador@example.test')).toHaveLength(2)
  })

  it('treats reinvite success as a neutral message when no users are cached', async () => {
    const email = 'invitado@example.test'
    server.use(
      http.post(`${base}/users/invitations`, () =>
        HttpResponse.json(
          buildUser({
            id: 'invited',
            email,
            name: null,
            role: 'admin',
            status: 'invited',
          }),
          { status: 200 },
        ),
      ),
    )
    renderInvite()
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), email)
    await user.click(screen.getByRole('radio', { name: /Administrador/ }))
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(
      await screen.findByRole('heading', { name: 'Usuarios' }),
    ).toBeVisible()
    expect(
      await screen.findByText(
        'invitado@example.test ya estaba invitado: le reenviamos la invitación.',
      ),
    ).toBeVisible()
  })

  it('announces reinviting a cached user with the newly selected role', async () => {
    const email = 'operador@example.test'
    const existing = buildUser({
      id: 'invited',
      email,
      name: null,
      role: 'operator',
      status: 'invited',
    })
    server.use(
      http.post(`${base}/users/invitations`, () =>
        HttpResponse.json({ ...existing, role: 'admin' }, { status: 200 }),
      ),
    )
    const { queryClient } = renderInvite(true, {
      configureQueryClient: (client) => {
        client.setQueryDefaults(queryKeys.users, { gcTime: 5 * 60_000 })
        client.setQueryData(queryKeys.users, [existing])
      },
    })
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), email)
    await user.click(screen.getByRole('radio', { name: /Administrador/ }))
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(
      await screen.findByText(
        'operador@example.test ya estaba invitado: le reenviamos la invitación como Administrador.',
      ),
    ).toBeVisible()
    expect(queryClient.getQueryState(queryKeys.users)?.isInvalidated).toBe(true)
  })

  it('does not announce a role change when a reinvited user keeps the same role', async () => {
    const email = 'operador@example.test'
    const existing = buildUser({
      id: 'invited',
      email,
      name: null,
      role: 'operator',
      status: 'invited',
    })
    server.use(
      http.post(`${base}/users/invitations`, () =>
        HttpResponse.json(existing, { status: 200 }),
      ),
    )
    renderInvite(true, {
      configureQueryClient: (client) => {
        client.setQueryData(queryKeys.users, [existing])
      },
    })
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), email)
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(
      await screen.findByText(
        'operador@example.test ya estaba invitado: le reenviamos la invitación.',
      ),
    ).toBeVisible()
    expect(screen.queryByText(/como Operador/)).not.toBeInTheDocument()
  })

  it('shows email_taken on the field, with the disabled user hint when cached', async () => {
    const disabled = buildUser({
      id: 'disabled',
      email: 'persona@example.test',
      name: 'Ana Pérez',
      status: 'disabled',
    })
    server.use(
      http.post(`${base}/users/invitations`, () =>
        HttpResponse.json(problem('email_taken', 409), {
          status: 409,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    renderInvite(true, {
      configureQueryClient: (client) => {
        client.setQueryDefaults(queryKeys.users, { gcTime: 5 * 60_000 })
        client.setQueryData(queryKeys.users, [disabled])
      },
    })
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), 'persona@example.test')
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(
      await screen.findByText(
        'Es de Ana Pérez, que está desactivado. Podés reactivarlo desde la lista.',
      ),
    ).toBeVisible()
    expect(screen.getByLabelText('Email')).toHaveAttribute(
      'aria-invalid',
      'true',
    )
  })

  it('shows the ordinary email_taken message when the email has no disabled user', async () => {
    server.use(
      http.post(`${base}/users/invitations`, () =>
        HttpResponse.json(problem('email_taken', 409), {
          status: 409,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    renderInvite()
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), 'ocupado@example.test')
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(
      await screen.findByText('Ese email ya tiene un usuario en el sistema.'),
    ).toBeVisible()
    expect(screen.getByLabelText('Email')).toHaveFocus()
  })

  it('maps a server role validation error to the role group', async () => {
    server.use(
      http.post(`${base}/users/invitations`, () =>
        HttpResponse.json(
          problem('validation_failed', 422, {
            errors: [{ field: 'role', code: 'invalid_value' }],
          }),
          {
            status: 422,
            headers: { 'Content-Type': 'application/problem+json' },
          },
        ),
      ),
    )
    renderInvite()
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), 'persona@example.test')
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(await screen.findByText('Elegí un rol.')).toBeVisible()
    expect(screen.getByRole('radiogroup', { name: 'Rol' })).toHaveAttribute(
      'aria-invalid',
      'true',
    )
  })

  it('shows the verification notice instead of the form until session refetch confirms the email', async () => {
    let verified = false
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: verified },
          }),
        ),
      ),
    )
    renderInvite(false)
    expect(
      screen.getByText('Confirmá tu email para invitar a tu equipo.'),
    ).toBeVisible()
    expect(screen.queryByLabelText('Email')).not.toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Volver a Usuarios' }),
    ).toHaveAttribute('href', '/settings/users')
    verified = true
    await act(async () => {
      focusManager.setFocused(false)
      focusManager.setFocused(true)
    })
    await waitFor(() =>
      expect(screen.getByRole('radio', { name: /Operador/ })).toBeChecked(),
    )
  })

  it('resends verification without posting an invitation while the email is unconfirmed', async () => {
    const resend = vi.fn(() => new HttpResponse(null, { status: 202 }))
    const invite = vi.fn()
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: false },
          }),
        ),
      ),
      http.post(`${base}/auth/email-verification/resend`, resend),
      http.post(`${base}/users/invitations`, invite),
    )
    renderInvite(false)
    await userEvent
      .setup()
      .click(
        screen.getByRole('button', { name: 'Reenviar email de confirmación' }),
      )
    expect(
      await screen.findByText(
        'Listo, te lo reenviamos. Revisá también el correo no deseado.',
      ),
    ).toBeVisible()
    expect(resend).toHaveBeenCalledOnce()
    expect(invite).not.toHaveBeenCalled()
  })

  it('handles a stale email verification cache by refreshing /me and showing the notice', async () => {
    let verified = false
    const me = vi.fn(() =>
      HttpResponse.json(
        buildSessionInfo({
          user: { ...buildSessionInfo().user, email_verified: verified },
        }),
      ),
    )
    const invite = vi.fn(() =>
      HttpResponse.json(problem('email_not_verified', 403), {
        status: 403,
        headers: { 'Content-Type': 'application/problem+json' },
      }),
    )
    server.use(
      http.get(`${base}/me`, me),
      http.post(`${base}/users/invitations`, invite),
    )
    const { queryClient } = renderInvite(true)
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), 'persona@example.test')
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(
      await screen.findByText('Confirmá tu email para invitar a tu equipo.'),
    ).toBeVisible()
    expect(
      screen.getByText(
        'Te mandamos un enlace cuando te registraste. Si no lo encontrás, pedí uno nuevo.',
      ),
    ).toBeVisible()
    expect(
      screen.getAllByText('Confirmá tu email para invitar a tu equipo.'),
    ).toHaveLength(1)
    expect(
      screen.getByRole('button', { name: 'Reenviar email de confirmación' }),
    ).toBeVisible()
    expect(
      screen.queryByText('No tenés permiso para hacer esto.'),
    ).not.toBeInTheDocument()
    expect(invite).toHaveBeenCalledOnce()
    expect(me).toHaveBeenCalled()
    expect(screen.queryByLabelText('Email')).not.toBeInTheDocument()
    verified = true
    await queryClient.refetchQueries({
      queryKey: queryKeys.session,
      exact: true,
      type: 'active',
    })
    await waitFor(() => expect(me).toHaveBeenCalledTimes(2))
    await waitFor(() =>
      expect(screen.getByLabelText('Email')).toHaveValue(
        'persona@example.test',
      ),
    )
    expect(screen.getByRole('radio', { name: /Operador/ })).toBeChecked()
    expect(
      document.getElementById('verify-to-invite-notice'),
    ).not.toBeInTheDocument()
  })

  it('keeps the invite form visible when an email_not_verified response is followed by a verified session', async () => {
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
      ),
      http.post(`${base}/users/invitations`, () =>
        HttpResponse.json(problem('email_not_verified', 403), {
          status: 403,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    renderInvite(true)
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), 'persona@example.test')
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(await screen.findByLabelText('Email')).toHaveValue(
      'persona@example.test',
    )
    expect(
      document.getElementById('verify-to-invite-notice'),
    ).not.toBeInTheDocument()
  })

  it('shows the role error beside the group and retains the form after a rate limit', async () => {
    const request = vi.fn(() =>
      HttpResponse.json(problem('rate_limited', 429), {
        status: 429,
        headers: {
          'Content-Type': 'application/problem+json',
          'Retry-After': '360',
        },
      }),
    )
    server.use(http.post(`${base}/users/invitations`, request))
    renderInvite()
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Email'), 'operador@example.test')
    await user.click(screen.getByRole('radio', { name: /Administrador/ }))
    await user.click(screen.getByRole('button', { name: 'Enviar invitación' }))
    expect(
      await screen.findByText('Por ahora no podemos enviar más invitaciones.'),
    ).toBeVisible()
    expect(
      screen.getByText(
        'Hay un límite de invitaciones y reactivaciones seguidas para cuidar el envío de emails. Esperá 6 minutos y probá de nuevo.',
      ),
    ).toBeVisible()
    expect(request).toHaveBeenCalledOnce()
    expect(screen.getByLabelText('Email')).toHaveValue('operador@example.test')
    expect(screen.getByRole('radio', { name: /Administrador/ })).toBeChecked()
  })
})
