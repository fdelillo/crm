import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { ResetPasswordPage } from './ResetPasswordPage'

const base = 'https://localhost:5173/api/v1'

describe('ResetPasswordPage', () => {
  // T-F403 (US-2.3, BR-F06, DD-F16): confirms a new password with the hidden link token and clears the cached session.
  it('sends the fragment token only in the request body and replaces with the changed-password login', async () => {
    const request = vi.fn(async ({ request: req }: { request: Request }) => {
      expect(new URL(req.url).hash).toBe('')
      expect(new URL(req.url).search).toBe('')
      expect(await req.json()).toEqual({ token: 'abc-_123', password: 'nueva-clave-segura' })
      return new HttpResponse(null, { status: 204 })
    })
    server.use(http.post(`${base}/auth/password-reset/confirm`, request))
    const { router, queryClient } = renderApp(['/reset-password#token=abc-_123'], {
      routes: [{ path: '/reset-password', Component: ResetPasswordPage }],
    })
    queryClient.setQueryData(['stale-session'], 'secret')
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Nueva contraseña'), 'nueva-clave-segura')
    expect(screen.getByLabelText('Nueva contraseña')).toHaveAttribute('autocomplete', 'new-password')
    await user.click(screen.getByRole('button', { name: 'Guardar contraseña' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(router.state.location.search).toBe('?reason=password_changed')
    expect(router.state.historyAction).toBe('REPLACE')
    expect(queryClient.getQueryData(['stale-session'])).toBeUndefined()
    expect(request).toHaveBeenCalledOnce()
  })

  it('shows the incomplete-link exit and validates short passwords without a request', async () => {
    const request = vi.fn()
    server.use(http.post(`${base}/auth/password-reset/confirm`, request))
    renderApp(['/reset-password'], { routes: [{ path: '/reset-password', Component: ResetPasswordPage }] })
    expect(screen.getByText('Este enlace está incompleto.')).toBeVisible()
    expect(screen.getByRole('link', { name: 'Pedir un enlace nuevo' })).toHaveAttribute('href', '/forgot-password')
    renderApp(['/reset-password#token=abc-_123'], { routes: [{ path: '/reset-password', Component: ResetPasswordPage }] })
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Nueva contraseña'), 'shortpass')
    await user.click(screen.getByRole('button', { name: 'Guardar contraseña' }))
    expect(await screen.findByText('La contraseña tiene que tener al menos 10 caracteres.')).toBeVisible()
    expect(request).not.toHaveBeenCalled()
  })

  it('shows a generic invalid-token explanation and a fresh-link action', async () => {
    server.use(http.post(`${base}/auth/password-reset/confirm`, () => HttpResponse.json({ type: '/problems/token_invalid', title: 'bad', status: 400, code: 'token_invalid' }, { status: 400, headers: { 'Content-Type': 'application/problem+json' } })))
    renderApp(['/reset-password#token=abc-_123'], { routes: [{ path: '/reset-password', Component: ResetPasswordPage }] })
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Nueva contraseña'), 'nueva-clave-segura')
    await user.click(screen.getByRole('button', { name: 'Guardar contraseña' }))
    expect(await screen.findByText('Este enlace ya no sirve.')).toBeVisible()
    expect(screen.getByText('Vence a la hora o ya se usó. Pedí uno nuevo.')).toBeVisible()
    await user.click(screen.getByRole('link', { name: 'Pedir un enlace nuevo' }))
  })
})
