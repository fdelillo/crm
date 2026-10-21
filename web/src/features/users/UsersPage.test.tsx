import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { UsersPage } from './pages'
import { buildSessionInfo, buildUser, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { queryKeys } from '@/api/queryKeys'

const base = 'https://localhost:5173/api/v1'

describe('UsersPage', () => {
  // T-F502 (US-3, FR-005, FR-007, DD-F25, DD-F29, P-F20): user-visible list and its states.
  it('shows three accessible skeletons while the users request is pending', async () => {
    server.use(http.get(`${base}/users`, () => new Promise(() => {})))
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } })),
    })
    const loading = screen.getByRole('region', { name: 'Cargando usuarios', busy: true })
    expect(loading).toBeVisible()
    expect(loading.querySelectorAll('[aria-hidden="true"]')).toHaveLength(3)
  })

  it('renders users and invitation expiry in the company time zone without truncating names', async () => {
    const longName = 'Nombre completo '.repeat(8)
    server.use(http.get(`${base}/users`, () => HttpResponse.json({ items: [
      buildUser({ id: 'me', name: 'Ana Pérez', email: 'ana@example.test', role: 'admin', status: 'active' }),
      buildUser({ id: 'invited', name: null, email: 'juan@example.test', role: 'operator', status: 'invited', invitation_expires_at: '2026-10-27T12:00:00Z' }),
      buildUser({ id: 'expired', name: null, email: 'pedro@example.test', role: 'operator', status: 'invited', invitation_expires_at: '2026-09-20T17:30:00Z' }),
      buildUser({ id: 'long', name: longName, email: 'luis@example.test', status: 'disabled' }),
    ] })))
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo({ user: { ...buildSessionInfo().user, id: 'me', name: 'Ana Pérez', email_verified: true } })),
    })
    expect(await screen.findByText('Ana Pérez (vos)')).toBeVisible()
    expect(screen.getAllByText('juan@example.test')).toHaveLength(2)
    expect(screen.getByText((_, element) => element?.textContent?.replace(/\s/g, '') === 'Administrador·Activo')).toBeVisible()
    expect(screen.getAllByText('Invitado')).toHaveLength(2)
    expect(screen.getAllByText('Operador')).toHaveLength(2)
    expect(screen.getAllByText((_, element) => element?.textContent?.includes('La invitación vence el 27/10, 09:00') ?? false).length).toBeGreaterThan(0)
    expect(screen.getAllByText((_, element) => element?.textContent?.includes('La invitación venció el 20/9, 14:30. Reenviala para que pueda entrar.') ?? false).length).toBeGreaterThan(0)
    expect(screen.getByText(longName.trim())).toBeVisible()
  })

  it('shows an empty state with an invite action when only the current user is returned', async () => {
    server.use(http.get(`${base}/users`, () => HttpResponse.json({ items: [buildUser()] })))
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } })),
    })
    expect(await screen.findByText('Todavía sos el único usuario.')).toBeVisible()
    expect(screen.getByRole('link', { name: 'Invitar' })).toHaveAttribute('href', '/settings/users/invite')
  })

  it('shows the section error, request code and retry action', async () => {
    const getUsers = vi.fn(() => HttpResponse.json(problem('internal', 500, { instance: '/requests/abc' }), { status: 500, headers: { 'Content-Type': 'application/problem+json' } }))
    server.use(http.get(`${base}/users`, getUsers))
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } })),
    })
    expect(await screen.findByText('No pudimos cargar los usuarios.')).toBeVisible()
    expect(screen.getByText(/Código:/)).toBeVisible()
    await screen.getByRole('button', { name: 'Reintentar' }).click()
    await waitFor(() => expect(getUsers).toHaveBeenCalledTimes(2))
  })

  it('gates invitation while email is unverified, gives the notice an empty live region, then enables on focus refetch', async () => {
    let confirmed = false
    server.use(
      http.get(`${base}/users`, () => HttpResponse.json({ items: [buildUser({ id: 'me' })] })),
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: confirmed } }))),
    )
    const { queryClient } = renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo()),
    })
    expect(await screen.findByText('Confirmá tu email para invitar a tu equipo.')).toBeVisible()
    const invite = screen.getByRole('button', { name: '+ Invitar' })
    expect(invite).toBeDisabled()
    expect(invite).toHaveAttribute('aria-describedby')
    expect(screen.getByText('Te mandamos un enlace a persona@example.test. Si no lo encontrás, pedí uno nuevo.')).toBeVisible()
    const liveRegion = screen.getByRole('status')
    expect(liveRegion).toBeEmptyDOMElement()
    confirmed = true
    await queryClient.refetchQueries({ queryKey: queryKeys.session, exact: true, type: 'active' })
    await waitFor(() => expect(screen.getByRole('link', { name: '+ Invitar' })).toBeEnabled())
  })

  it('uses the shared email verification resend flow and announces its result in the existing region', async () => {
    const resendRequest = vi.fn(() => new HttpResponse(null, { status: 202 }))
    server.use(
      http.get(`${base}/users`, () => HttpResponse.json({ items: [buildUser()] })),
      http.post(`${base}/auth/email-verification/resend`, resendRequest),
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo()),
    })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Reenviar email de confirmación' }))
    expect(await screen.findByText('Listo, te lo reenviamos. Revisá también el correo no deseado.')).toBeVisible()
    expect(resendRequest).toHaveBeenCalledOnce()
    expect(screen.getAllByRole('status')).toHaveLength(1)
  })

  // T-F503 (US-3.3, US-3.4, DD-F7, DD-F8, INV-F10): actions stay keyboard-operable and confirmed.
  it('opens invited-user actions from the keyboard and updates an invited operator to administrator', async () => {
    let invitedRole: 'operator' | 'admin' = 'operator'
    const invited = buildUser({ id: 'invited', name: null, email: 'equipo@example.test', role: 'operator', status: 'invited', invitation_expires_at: '2026-10-27T12:00:00Z' })
    server.use(
      http.get(`${base}/users`, () => HttpResponse.json({ items: [buildUser({ role: 'admin', email_verified: true }), { ...invited, role: invitedRole }] })),
      http.put(`${base}/users/${invited.id}/role`, async ({ request }) => {
        expect(await request.json()).toEqual({ role: 'admin' })
        invitedRole = 'admin'
        return HttpResponse.json({ ...invited, role: invitedRole })
      }),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } })),
    })
    const user = userEvent.setup()
    const menuButton = await screen.findByRole('button', { name: 'Acciones para equipo@example.test' })
    menuButton.focus()
    await user.keyboard('{Enter}')
    expect(screen.getByRole('menuitem', { name: 'Reenviar invitación' })).toBeVisible()
    expect(screen.getByRole('menuitem', { name: 'Hacer Administrador' })).toBeVisible()
    expect(screen.getByRole('menuitem', { name: 'Desactivar' })).toBeVisible()
    await user.click(screen.getByRole('menuitem', { name: 'Hacer Administrador' }))
    expect(screen.getByRole('alertdialog')).toBeVisible()
    expect(screen.getByText('Cuando acepte la invitación, va a poder ver toda la información de la empresa y gestionar usuarios.')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Hacer Administrador' }))
    expect(await screen.findByText('equipo@example.test ahora es Administrador')).toBeVisible()
    await waitFor(() => expect(screen.getAllByText('Administrador')).toHaveLength(2))
    await waitFor(() => expect(menuButton).toHaveFocus())
  })

  it('cancels a confirmation without a request and returns keyboard focus to the row action', async () => {
    const invited = buildUser({ id: 'invited', name: null, email: 'equipo@example.test', role: 'operator', status: 'invited' })
    const changeRequest = vi.fn()
    server.use(
      http.get(`${base}/users`, () => HttpResponse.json({ items: [buildUser({ role: 'admin', email_verified: true }), invited] })),
      http.put(`${base}/users/${invited.id}/role`, changeRequest),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } })),
    })
    const user = userEvent.setup()
    const menuButton = await screen.findByRole('button', { name: 'Acciones para equipo@example.test' })
    await user.click(menuButton)
    await user.click(screen.getByRole('menuitem', { name: 'Hacer Administrador' }))
    await user.keyboard('{Escape}')
    await waitFor(() => expect(menuButton).toHaveFocus())
    expect(changeRequest).not.toHaveBeenCalled()
  })

  it('disables both destructive actions for the only active administrator', async () => {
    server.use(http.get(`${base}/users`, () => HttpResponse.json({ items: [buildUser({ role: 'admin', email_verified: true })] })))
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } })),
    })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Acciones para Persona de prueba' }))
    const makeOperator = screen.getByRole('menuitem', { name: /Cambiar a Operador/ })
    const deactivate = screen.getByRole('menuitem', { name: /Desactivar/ })
    expect(makeOperator).toHaveAttribute('aria-disabled', 'true')
    expect(deactivate).toHaveAttribute('aria-disabled', 'true')
    expect(screen.getAllByText('Tiene que quedar al menos un administrador activo')).toHaveLength(2)
  })

  it('shows only Reactivar for a disabled user and announces successful reactivation', async () => {
    let status: 'disabled' | 'active' = 'disabled'
    const disabled = buildUser({ id: 'disabled', name: 'Lucía', email: 'lucia@example.test', status })
    server.use(
      http.get(`${base}/users`, () => HttpResponse.json({ items: [buildUser({ role: 'admin', email_verified: true }), { ...disabled, status }] })),
      http.post(`${base}/users/${disabled.id}/reactivate`, () => { status = 'active'; return HttpResponse.json({ ...disabled, status }) }),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } })),
    })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Acciones para Lucía' }))
    expect(screen.getByRole('menuitem', { name: 'Reactivar' })).toBeVisible()
    expect(screen.queryByRole('menuitem', { name: 'Hacer Administrador' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('menuitem', { name: 'Reactivar' }))
    expect(screen.getByRole('alertdialog')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Reactivar' }))
    expect(await screen.findByText('Reactivaste a Lucía')).toBeVisible()
    await waitFor(() => expect(screen.getAllByText('Activo')).toHaveLength(2))
  })

  it('announces invitation rate limits without requesting the users list again', async () => {
    const invited = buildUser({ id: 'invited', name: null, email: 'equipo@example.test', role: 'operator', status: 'invited' })
    const getUsers = vi.fn(() => HttpResponse.json({ items: [buildUser({ role: 'admin', email_verified: true }), invited] }))
    server.use(
      http.get(`${base}/users`, getUsers),
      http.post(`${base}/users/invitations`, () => HttpResponse.json(problem('rate_limited', 429), { status: 429, headers: { 'Content-Type': 'application/problem+json', 'Retry-After': '360' } })),
    )
    renderApp(['/settings/users'], {
      routes: [{ path: '/settings/users', Component: UsersPage }],
      configureQueryClient: (client) => client.setQueryData(queryKeys.session, buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } })),
    })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Acciones para equipo@example.test' }))
    await user.click(screen.getByRole('menuitem', { name: 'Reenviar invitación' }))
    expect(await screen.findByText(/Por ahora no podemos enviar más invitaciones/)).toBeVisible()
    expect(screen.getByText(/Esperá 6 minutos y probá de nuevo/)).toBeVisible()
    expect(getUsers).toHaveBeenCalledTimes(1)
  })
})
