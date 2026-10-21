import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { UsersPage } from './pages'
import { buildSessionInfo, buildUser, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { queryKeys } from '@/api/queryKeys'
import { focusManager } from '@tanstack/react-query'
import type { SessionInfo } from '@/api/types'

const base = 'https://localhost:5173/api/v1'

describe('UsersPage', () => {
  afterEach(() => focusManager.setFocused(undefined))
  // T-F502 (US-3, FR-005, FR-007, DD-F25, DD-F29, P-F20): user-visible list and its states.
  it('shows three accessible skeletons while the users request is pending', async () => {
    server.use(http.get(`${base}/users`, () => new Promise(() => {})))
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const loading = screen.getByRole('region', {
      name: 'Cargando usuarios',
      busy: true,
    })
    expect(loading).toBeVisible()
    expect(loading.querySelectorAll('[aria-hidden="true"]')).toHaveLength(3)
  })

  it('renders users and invitation expiry in the company time zone without truncating names', async () => {
    const longName = 'Nombre completo '.repeat(8)
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
      ),
      http.get(`${base}/users`, () =>
        HttpResponse.json({
          items: [
            buildUser({
              id: 'me',
              name: 'Ana Pérez',
              email: 'ana@example.test',
              role: 'admin',
              status: 'active',
            }),
            buildUser({
              id: 'invited',
              name: null,
              email: 'juan@example.test',
              role: 'operator',
              status: 'invited',
              invitation_expires_at: '2026-10-27T12:00:00Z',
            }),
            buildUser({
              id: 'expired',
              name: null,
              email: 'pedro@example.test',
              role: 'operator',
              status: 'invited',
              invitation_expires_at: '2026-09-20T17:30:00Z',
            }),
            buildUser({
              id: 'no-expiry',
              name: null,
              email: 'no-expiry@example.test',
              role: 'operator',
              status: 'invited',
              invitation_expires_at: null,
            }),
            buildUser({
              id: 'long',
              name: longName,
              email: 'luis@example.test',
              status: 'disabled',
            }),
          ],
        }),
      ),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: {
              ...buildSessionInfo().user,
              id: 'me',
              name: 'Ana Pérez',
              email_verified: true,
            },
          }),
        ),
    })
    expect(await screen.findByText('Ana Pérez (vos)')).toBeVisible()
    expect(screen.getAllByText('juan@example.test')).toHaveLength(2)
    expect(
      screen.getByText(
        (_, element) =>
          element?.textContent?.replace(/\s/g, '') === 'Administrador·Activo',
      ),
    ).toBeVisible()
    expect(screen.getAllByText('Invitado')).toHaveLength(3)
    expect(screen.getAllByText('Operador')).toHaveLength(3)
    const noExpiryRow = screen
      .getAllByText('no-expiry@example.test')[0]
      ?.closest('li')
    expect(noExpiryRow).not.toBeNull()
    expect(
      within(noExpiryRow as HTMLElement).queryByText(/La invitación vence/),
    ).toBeNull()
    expect(
      screen.getAllByText(
        (_, element) =>
          element?.textContent?.includes(
            'La invitación vence el 27/10, 09:00',
          ) ?? false,
      ).length,
    ).toBeGreaterThan(0)
    expect(
      screen.getAllByText(
        (_, element) =>
          element?.textContent?.includes(
            'La invitación venció el 20/9, 14:30. Reenviala para que pueda entrar.',
          ) ?? false,
      ).length,
    ).toBeGreaterThan(0)
    expect(screen.getByText(longName.trim())).toBeVisible()
  })

  it('shows an empty state with an invite action when only the current user is returned', async () => {
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({ items: [buildUser()] }),
      ),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    expect(
      await screen.findByText('Todavía sos el único usuario.'),
    ).toBeVisible()
    expect(screen.getByRole('link', { name: 'Invitar' })).toHaveAttribute(
      'href',
      '/settings/users/invite',
    )
  })

  it('disables the empty-state invite action and connects it to the verification notice', async () => {
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({ items: [buildUser()] }),
      ),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(queryKeys.session, buildSessionInfo()),
    })
    const notice = await screen.findByText(
      'Confirmá tu email para invitar a tu equipo.',
    )
    const action = await screen.findByRole('button', { name: 'Invitar' })
    expect(action).toBeDisabled()
    expect(action).toHaveAttribute(
      'aria-describedby',
      'verify-to-invite-notice',
    )
    expect(document.getElementById('verify-to-invite-notice')).toContainElement(
      notice,
    )
  })

  it('shows the section error, request code and retry action', async () => {
    const getUsers = vi.fn(() =>
      HttpResponse.json(
        problem('internal', 500, { instance: '/requests/abc' }),
        {
          status: 500,
          headers: { 'Content-Type': 'application/problem+json' },
        },
      ),
    )
    server.use(http.get(`${base}/users`, getUsers))
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    expect(
      await screen.findByText('No pudimos cargar los usuarios.'),
    ).toBeVisible()
    expect(screen.getByText(/Código:/)).toBeVisible()
    await screen.getByRole('button', { name: 'Reintentar' }).click()
    await waitFor(() => expect(getUsers).toHaveBeenCalledTimes(2))
  })

  it('gates invitation while email is unverified, gives the notice an empty live region, then enables on focus refetch', async () => {
    let confirmed = false
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({
          items: [
            buildUser(),
            buildUser({
              id: 'invited',
              name: null,
              email: 'equipo@example.test',
              role: 'operator',
              status: 'invited',
            }),
          ],
        }),
      ),
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: confirmed },
          }),
        ),
      ),
    )
    const { queryClient } = renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(queryKeys.session, buildSessionInfo()),
    })
    expect(
      await screen.findByText('Confirmá tu email para invitar a tu equipo.'),
    ).toBeVisible()
    const invite = screen.getByRole('button', { name: '+ Invitar' })
    expect(invite).toBeDisabled()
    expect(invite).toHaveAttribute(
      'aria-describedby',
      'verify-to-invite-notice',
    )
    expect(
      screen.getByText(
        'Te mandamos un enlace a persona@example.test. Si no lo encontrás, pedí uno nuevo.',
      ),
    ).toBeVisible()
    const liveRegion = screen.getByRole('status')
    expect(liveRegion).toBeEmptyDOMElement()
    await userEvent.setup().click(
      await screen.findByRole('button', {
        name: 'Acciones para equipo@example.test',
      }),
    )
    const resendInvitation = screen.getByRole('menuitem', {
      name: /Reenviar invitación/,
    })
    expect(resendInvitation).toHaveAttribute('aria-disabled', 'true')
    await userEvent.setup().keyboard('{Escape}')
    confirmed = true
    await act(async () => {
      focusManager.setFocused(false)
      focusManager.setFocused(true)
    })
    await waitFor(() =>
      expect(
        queryClient.getQueryData<SessionInfo>(queryKeys.session)?.user
          .email_verified,
      ).toBe(true),
    )
    await waitFor(() =>
      expect(screen.getByRole('link', { name: '+ Invitar' })).toBeEnabled(),
    )
  })

  it('uses the shared email verification resend flow and announces its result in the existing region', async () => {
    const resendRequest = vi.fn(() => new HttpResponse(null, { status: 202 }))
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({ items: [buildUser()] }),
      ),
      http.post(`${base}/auth/email-verification/resend`, resendRequest),
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(queryKeys.session, buildSessionInfo()),
    })
    const user = userEvent.setup()
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
    expect(resendRequest).toHaveBeenCalledOnce()
    expect(screen.getAllByRole('status')).toHaveLength(1)
  })

  it.each([
    [120, '2 minutos'],
    [1200, '20 minutos'],
  ])(
    'shows the resend rate limit from the API for %i seconds in the same live region',
    async (seconds, wait) => {
      server.use(
        http.get(`${base}/users`, () =>
          HttpResponse.json({ items: [buildUser()] }),
        ),
        http.post(`${base}/auth/email-verification/resend`, () =>
          HttpResponse.json(problem('rate_limited', 429), {
            status: 429,
            headers: {
              'Content-Type': 'application/problem+json',
              'Retry-After': String(seconds),
            },
          }),
        ),
      )
      renderApp(['/settings/users'], {
        routes: [{ path: '/settings/users', Component: UsersPage }],
        configureQueryClient: (client) =>
          client.setQueryData(queryKeys.session, buildSessionInfo()),
      })
      const region = await screen.findByRole('status')
      expect(region).toBeEmptyDOMElement()
      await userEvent.setup().click(
        screen.getByRole('button', {
          name: 'Reenviar email de confirmación',
        }),
      )
      expect(
        await screen.findByText('Ya te lo mandamos hace poco.'),
      ).toBeVisible()
      expect(screen.getByText(`Esperá ${wait} y probá de nuevo.`)).toBeVisible()
      expect(screen.getByRole('status')).toBe(region)
      expect(region).toHaveTextContent(`Esperá ${wait} y probá de nuevo.`)
    },
  )

  it('rechecks /me and disables invitation actions after S-09 receives email_not_verified', async () => {
    let verified = true
    const me = vi.fn(() =>
      HttpResponse.json(
        buildSessionInfo({
          user: { ...buildSessionInfo().user, email_verified: verified },
        }),
      ),
    )
    const invited = buildUser({
      id: 'invited',
      name: null,
      email: 'equipo@example.test',
      role: 'operator',
      status: 'invited',
    })
    server.use(
      http.get(`${base}/me`, me),
      http.get(`${base}/users`, () =>
        HttpResponse.json({ items: [buildUser(), invited] }),
      ),
      http.post(`${base}/users/invitations`, () => {
        verified = false
        return HttpResponse.json(problem('email_not_verified', 403), {
          status: 403,
          headers: { 'Content-Type': 'application/problem+json' },
        })
      }),
    )
    const { queryClient } = renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', {
        name: 'Acciones para equipo@example.test',
      }),
    )
    await user.click(
      screen.getByRole('menuitem', { name: 'Reenviar invitación' }),
    )
    await waitFor(() =>
      expect(
        queryClient.getQueryData<SessionInfo>(queryKeys.session)?.user
          .email_verified,
      ).toBe(false),
    )
    expect(
      document.getElementById('verify-to-invite-notice'),
    ).toHaveTextContent('Confirmá tu email para invitar a tu equipo.')
    expect(me).toHaveBeenCalled()
    await user.click(
      screen.getByRole('button', { name: 'Acciones para equipo@example.test' }),
    )
    expect(
      screen.getByRole('menuitem', { name: /Reenviar invitación/ }),
    ).toHaveAttribute('aria-disabled', 'true')
  })

  it('focuses the page heading when verification resend finds the email confirmed', async () => {
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({ items: [buildUser()] }),
      ),
      http.post(
        `${base}/auth/email-verification/resend`,
        () => new HttpResponse(null, { status: 202 }),
      ),
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
      ),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(queryKeys.session, buildSessionInfo()),
    })
    const heading = await screen.findByRole('heading', { name: 'Usuarios' })
    await userEvent
      .setup()
      .click(
        screen.getByRole('button', { name: 'Reenviar email de confirmación' }),
      )
    expect(
      await screen.findByText(
        'Tu email ya está confirmado. No hace falta otro enlace.',
      ),
    ).toBeVisible()
    expect(document.getElementById('verify-to-invite-notice')).toBeNull()
    expect(screen.getByRole('link', { name: '+ Invitar' })).toBeVisible()
    expect(heading).toHaveFocus()
    expect(
      screen.queryByText(
        'Listo, te lo reenviamos. Revisá también el correo no deseado.',
      ),
    ).not.toBeInTheDocument()
  })

  // T-F503 (US-3.3, US-3.4, DD-F7, DD-F8, INV-F10): actions stay keyboard-operable and confirmed.
  it('opens invited-user actions from the keyboard and updates an invited operator to administrator', async () => {
    let invitedRole: 'operator' | 'admin' = 'operator'
    const invited = buildUser({
      id: 'invited',
      name: null,
      email: 'equipo@example.test',
      role: 'operator',
      status: 'invited',
      invitation_expires_at: '2026-10-27T12:00:00Z',
    })
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({
          items: [
            buildUser({ role: 'admin', email_verified: true }),
            { ...invited, role: invitedRole },
          ],
        }),
      ),
      http.put(`${base}/users/${invited.id}/role`, async ({ request }) => {
        expect(await request.json()).toEqual({ role: 'admin' })
        invitedRole = 'admin'
        return HttpResponse.json({ ...invited, role: invitedRole })
      }),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    const menuButton = await screen.findByRole('button', {
      name: 'Acciones para equipo@example.test',
    })
    menuButton.focus()
    await user.keyboard('{Enter}')
    expect(
      screen.getByRole('menuitem', { name: 'Reenviar invitación' }),
    ).toBeVisible()
    expect(
      screen.getByRole('menuitem', { name: 'Hacer Administrador' }),
    ).toBeVisible()
    expect(screen.getByRole('menuitem', { name: 'Desactivar' })).toBeVisible()
    await user.keyboard('{ArrowDown}')
    expect(
      screen.getByRole('menuitem', { name: 'Hacer Administrador' }),
    ).toHaveFocus()
    await user.click(
      screen.getByRole('menuitem', { name: 'Hacer Administrador' }),
    )
    expect(screen.getByRole('alertdialog')).toBeVisible()
    expect(
      screen.getByText(
        'Cuando acepte la invitación, va a poder ver toda la información de la empresa y gestionar usuarios.',
      ),
    ).toBeVisible()
    await user.click(
      screen.getByRole('button', { name: 'Hacer Administrador' }),
    )
    expect(
      await screen.findByText('equipo@example.test ahora es Administrador'),
    ).toBeVisible()
    await waitFor(() =>
      expect(screen.getAllByText('Administrador')).toHaveLength(2),
    )
    await waitFor(() => expect(menuButton).toHaveFocus())
  })

  it('cancels a confirmation without a request and returns keyboard focus to the row action', async () => {
    const invited = buildUser({
      id: 'invited',
      name: null,
      email: 'equipo@example.test',
      role: 'operator',
      status: 'invited',
    })
    const changeRequest = vi.fn()
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({
          items: [buildUser({ role: 'admin', email_verified: true }), invited],
        }),
      ),
      http.put(`${base}/users/${invited.id}/role`, changeRequest),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    const menuButton = await screen.findByRole('button', {
      name: 'Acciones para equipo@example.test',
    })
    await user.click(menuButton)
    await user.click(
      screen.getByRole('menuitem', { name: 'Hacer Administrador' }),
    )
    await user.click(screen.getByRole('button', { name: 'Cancelar' }))
    await waitFor(() => expect(menuButton).toHaveFocus())
    expect(changeRequest).not.toHaveBeenCalled()
  })

  it('disables both destructive actions for the only active administrator', async () => {
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({
          items: [buildUser({ role: 'admin', email_verified: true })],
        }),
      ),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', {
        name: 'Acciones para Persona de prueba',
      }),
    )
    const makeOperator = screen.getByRole('menuitem', {
      name: /Cambiar a Operador/,
    })
    const deactivate = screen.getByRole('menuitem', { name: /Desactivar/ })
    expect(makeOperator).toHaveAttribute('aria-disabled', 'true')
    expect(deactivate).toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getAllByText('Tiene que quedar al menos un administrador activo'),
    ).toHaveLength(2)
  })

  it('shows only Reactivar for a disabled user and announces successful reactivation', async () => {
    let status: 'disabled' | 'active' = 'disabled'
    const disabled = buildUser({
      id: 'disabled',
      name: 'Lucía',
      email: 'lucia@example.test',
      status,
    })
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({
          items: [
            buildUser({ role: 'admin', email_verified: true }),
            { ...disabled, status },
          ],
        }),
      ),
      http.post(`${base}/users/${disabled.id}/reactivate`, () => {
        status = 'active'
        return HttpResponse.json({ ...disabled, status })
      }),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Acciones para Lucía' }),
    )
    expect(screen.getByRole('menuitem', { name: 'Reactivar' })).toBeVisible()
    expect(
      screen.queryByRole('menuitem', { name: 'Hacer Administrador' }),
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('menuitem', { name: 'Reactivar' }))
    expect(screen.getByRole('alertdialog')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Reactivar' }))
    expect(await screen.findByText('Reactivaste a Lucía')).toBeVisible()
    await waitFor(() => expect(screen.getAllByText('Activo')).toHaveLength(2))
  })

  it('announces invitation rate limits without requesting the users list again', async () => {
    const invited = buildUser({
      id: 'invited',
      name: null,
      email: 'equipo@example.test',
      role: 'operator',
      status: 'invited',
    })
    const getUsers = vi.fn(() =>
      HttpResponse.json({
        items: [buildUser({ role: 'admin', email_verified: true }), invited],
      }),
    )
    server.use(
      http.get(`${base}/users`, getUsers),
      http.post(`${base}/users/invitations`, () =>
        HttpResponse.json(problem('rate_limited', 429), {
          status: 429,
          headers: {
            'Content-Type': 'application/problem+json',
            'Retry-After': '360',
          },
        }),
      ),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', {
        name: 'Acciones para equipo@example.test',
      }),
    )
    await user.click(
      screen.getByRole('menuitem', { name: 'Reenviar invitación' }),
    )
    expect(
      await screen.findByText(/Por ahora no podemos enviar más invitaciones/),
    ).toBeVisible()
    expect(screen.getByText(/Esperá 6 minutos y probá de nuevo/)).toBeVisible()
    expect(getUsers).toHaveBeenCalledTimes(1)
  })

  it('keeps another user’s confirmation available while a resend is still pending', async () => {
    const invited = buildUser({
      id: 'invited',
      email: 'equipo@example.test',
      name: null,
      role: 'operator',
      status: 'invited',
    })
    const otherAdmin = buildUser({
      id: 'other-admin',
      email: 'otra@example.test',
      name: 'Otra Admin',
      role: 'admin',
      status: 'active',
    })
    let finishResend!: (response: Response) => void
    const resendResponse = new Promise<Response>((resolve) => {
      finishResend = resolve
    })
    const deactivate = vi.fn(() =>
      HttpResponse.json({ ...otherAdmin, status: 'disabled' }),
    )
    const resend = vi.fn(() => resendResponse)
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({ items: [buildUser(), invited, otherAdmin] }),
      ),
      http.post(`${base}/users/invitations`, resend),
      http.post(`${base}/users/${otherAdmin.id}/deactivate`, deactivate),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', {
        name: 'Acciones para equipo@example.test',
      }),
    )
    await user.click(
      screen.getByRole('menuitem', { name: 'Reenviar invitación' }),
    )
    await waitFor(() => expect(resend).toHaveBeenCalledOnce())
    await user.click(
      screen.getByRole('button', { name: 'Acciones para Otra Admin' }),
    )
    await user.click(screen.getByRole('menuitem', { name: 'Desactivar' }))
    expect(screen.getByRole('alertdialog')).toBeVisible()
    finishResend(
      HttpResponse.json(
        { ...invited, invitation_expires_at: '2026-10-27T12:00:00Z' },
        { status: 200 },
      ),
    )
    expect(
      await screen.findByText('Reenviamos la invitación a equipo@example.test'),
    ).toBeVisible()
    expect(screen.getByRole('alertdialog')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Desactivar' }))
    expect(await screen.findByText('Desactivaste a Otra Admin')).toBeVisible()
    expect(deactivate).toHaveBeenCalledOnce()
  })

  it('sends one request when the confirmation button is double-clicked', async () => {
    const invited = buildUser({
      id: 'invited',
      name: null,
      email: 'equipo@example.test',
      role: 'operator',
      status: 'invited',
    })
    let finishChange!: (response: Response) => void
    const response = new Promise<Response>((resolve) => {
      finishChange = resolve
    })
    const change = vi.fn(() => response)
    server.use(
      http.get(`${base}/users`, () =>
        HttpResponse.json({ items: [buildUser(), invited] }),
      ),
      http.put(`${base}/users/${invited.id}/role`, change),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', {
        name: 'Acciones para equipo@example.test',
      }),
    )
    await user.click(
      screen.getByRole('menuitem', { name: 'Hacer Administrador' }),
    )
    const confirm = screen.getByRole('button', { name: 'Hacer Administrador' })
    fireEvent.click(confirm)
    fireEvent.click(confirm)
    await waitFor(() => expect(change).toHaveBeenCalledOnce())
    finishChange(HttpResponse.json({ ...invited, role: 'admin' }))
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument(),
    )
    expect(change).toHaveBeenCalledOnce()
  })

  it('shows last-admin conflict once and refreshes the user list', async () => {
    const target = buildUser({
      id: 'other-admin',
      name: 'Otra Admin',
      email: 'otra@example.test',
      role: 'admin',
      status: 'active',
    })
    const getUsers = vi.fn(() =>
      HttpResponse.json({ items: [buildUser(), target] }),
    )
    server.use(
      http.get(`${base}/users`, getUsers),
      http.post(`${base}/users/${target.id}/deactivate`, () =>
        HttpResponse.json(problem('last_admin', 409), {
          status: 409,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Acciones para Otra Admin' }),
    )
    await user.click(screen.getByRole('menuitem', { name: 'Desactivar' }))
    await user.click(screen.getByRole('button', { name: 'Desactivar' }))
    expect(
      await screen.findByText(
        /Tiene que quedar al menos un administrador activo/,
      ),
    ).toBeVisible()
    await waitFor(() => expect(getUsers).toHaveBeenCalledTimes(2))
  })

  it('shows only the global forbidden message for a denied user action', async () => {
    const target = buildUser({
      id: 'other-admin',
      name: 'Otra Admin',
      email: 'otra@example.test',
      role: 'admin',
      status: 'active',
    })
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
      ),
      http.get(`${base}/users`, () =>
        HttpResponse.json({ items: [buildUser(), target] }),
      ),
      http.post(`${base}/users/${target.id}/deactivate`, () =>
        HttpResponse.json(problem('forbidden', 403), {
          status: 403,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Acciones para Otra Admin' }),
    )
    await user.click(screen.getByRole('menuitem', { name: 'Desactivar' }))
    await user.click(screen.getByRole('button', { name: 'Desactivar' }))
    expect(
      await screen.findByText('No tenés permiso para hacer esto.'),
    ).toBeVisible()
    expect(
      screen.getAllByText('No tenés permiso para hacer esto.'),
    ).toHaveLength(1)
  })

  it('keeps a disabled user after reactivation is rate-limited and restores row focus', async () => {
    const disabled = buildUser({
      id: 'disabled',
      name: 'Lucía',
      email: 'lucia@example.test',
      status: 'disabled',
    })
    const getUsers = vi.fn(() =>
      HttpResponse.json({ items: [buildUser(), disabled] }),
    )
    server.use(
      http.get(`${base}/users`, getUsers),
      http.post(`${base}/users/${disabled.id}/reactivate`, () =>
        HttpResponse.json(problem('rate_limited', 429), {
          status: 429,
          headers: {
            'Content-Type': 'application/problem+json',
            'Retry-After': '180',
          },
        }),
      ),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) =>
        client.setQueryData(
          queryKeys.session,
          buildSessionInfo({
            user: { ...buildSessionInfo().user, email_verified: true },
          }),
        ),
    })
    const user = userEvent.setup()
    const trigger = await screen.findByRole('button', {
      name: 'Acciones para Lucía',
    })
    await user.click(trigger)
    await user.click(screen.getByRole('menuitem', { name: 'Reactivar' }))
    await user.click(screen.getByRole('button', { name: 'Reactivar' }))
    expect(
      await screen.findByText(/Esperá 3 minutos y probá de nuevo/),
    ).toBeVisible()
    await waitFor(() => expect(trigger).toHaveFocus())
    expect(screen.getByText('Desactivado')).toBeVisible()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(getUsers).toHaveBeenCalledOnce()
  })
})
