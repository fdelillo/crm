import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { buildSessionInfo } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { VerifyEmailPage } from './VerifyEmailPage'

const base = 'https://localhost:5173/api/v1'
const problem = (code: string, status: number) => HttpResponse.json({ type: `/problems/${code}`, title: 'problem', status, code }, { status, headers: { 'Content-Type': 'application/problem+json' } })

describe('VerifyEmailPage', () => {
  // T-F404 (US-1.3, DD-13, DD-F3, P-F10): confirmation is an explicit user action with the account-safety warning.
  it('does not confirm on page load and confirms only after the user presses the button', async () => {
    const confirm = vi.fn(() => new HttpResponse(null, { status: 204 }))
    server.use(http.post(`${base}/auth/email-verification/confirm`, confirm))
    server.use(http.get(`${base}/me`, () => problem('unauthenticated', 401)))
    renderApp(['/verify-email#token=verify-_123'], { routes: [{ path: '/verify-email', Component: VerifyEmailPage }] })
    expect(await screen.findByText(/Si no creaste una cuenta con este email/)).toBeVisible()
    expect(confirm).not.toHaveBeenCalled()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Confirmar mi email' }))
    expect(await screen.findByText('¡Listo! Tu email quedó confirmado.')).toBeVisible()
    expect(screen.getByRole('link', { name: 'Ingresar' })).toBeVisible()
    expect(confirm).toHaveBeenCalledOnce()
  })

  it('offers resend only for an unverified signed-in session after an invalid link', async () => {
    const me = vi.fn(() => HttpResponse.json(buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: false } })))
    server.use(
      http.get(`${base}/me`, me),
      http.post(`${base}/auth/email-verification/confirm`, () => problem('token_invalid', 400)),
    )
    renderApp(['/verify-email#token=bad-token'], { routes: [{ path: '/verify-email', Component: VerifyEmailPage }] })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Confirmar mi email' }))
    expect(await screen.findByText('Este enlace de confirmación ya no sirve.')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Reenviar email de confirmación' })).toBeVisible()
    expect(me).toHaveBeenCalled()
  })

  it('shows the confirmed-session state without a resend action when token is missing', async () => {
    server.use(http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } }))))
    renderApp(['/verify-email'], { routes: [{ path: '/verify-email', Component: VerifyEmailPage }] })
    await waitFor(() => expect(screen.getByText('Tu email ya está confirmado. No hace falta otro enlace.')).toBeVisible())
    expect(screen.queryByRole('button', { name: 'Reenviar email de confirmación' })).not.toBeInTheDocument()
  })

  it('does not offer resend for an invalid token when the session is already verified', async () => {
    server.use(
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: true } }))),
      http.post(`${base}/auth/email-verification/confirm`, () => problem('token_invalid', 400)),
    )
    renderApp(['/verify-email#token=expired-token'], { routes: [{ path: '/verify-email', Component: VerifyEmailPage }] })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Confirmar mi email' }))
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Confirmá tu email' })).toBeVisible())
    expect(screen.queryByRole('button', { name: 'Reenviar email de confirmación' })).not.toBeInTheDocument()
  })

  it('uses the follow-up /me result after resend and never reports sent when it says verified', async () => {
    let getCalls = 0
    server.use(
      http.get(`${base}/me`, () => {
        getCalls += 1
        return HttpResponse.json(buildSessionInfo({ user: { ...buildSessionInfo().user, email_verified: getCalls > 1 } }))
      }),
      http.post(`${base}/auth/email-verification/confirm`, () => problem('token_invalid', 400)),
      http.post(`${base}/auth/email-verification/resend`, () => new HttpResponse(null, { status: 202 })),
    )
    renderApp(['/verify-email#token=expired-token'], { routes: [{ path: '/verify-email', Component: VerifyEmailPage }] })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Confirmar mi email' }))
    await user.click(await screen.findByRole('button', { name: 'Reenviar email de confirmación' }))
    await waitFor(() => expect(screen.getAllByText('Tu email ya está confirmado. No hace falta otro enlace.').length).toBeGreaterThanOrEqual(1))
    expect(screen.queryByText(/Listo, te lo reenviamos/)).not.toBeInTheDocument()
    expect(getCalls).toBe(2)
  })
})
