import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { buildSessionInfo, problem } from '@/test/fixtures'
import { ForgotPasswordPage } from './ForgotPasswordPage'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'

const base = 'https://localhost:5173/api/v1'

describe('ForgotPasswordPage', () => {
  // T-F402 (US-2.3, FR-004, INV-13, DD-20): requests a neutral reset link without exposing account existence.
  it('prefills the email, shows neutral success, and offers a prefilled retry', async () => {
    const request = vi.fn(() => HttpResponse.json(null, { status: 202 }))
    server.use(http.post(`${base}/auth/password-reset`, request))
    renderApp(
      [{ pathname: '/forgot-password', state: { email: 'ana@example.test' } }],
      {
        routes: [{ path: '/forgot-password', Component: ForgotPasswordPage }],
      },
    )
    expect(await screen.findByLabelText('Email')).toHaveValue(
      'ana@example.test',
    )
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Enviar enlace' }))
    expect(
      await screen.findByRole('heading', { name: 'Revisá tu email' }),
    ).toHaveFocus()
    expect(
      screen.getByText(/Si ana@example.test tiene un usuario/),
    ).toBeVisible()
    expect(screen.getByText(/esperá unos 20 minutos/)).toBeVisible()
    await user.click(
      screen.getByRole('button', { name: '¿No llegó? Pedilo de nuevo' }),
    )
    expect(screen.getByLabelText('Email')).toHaveValue('ana@example.test')
    await waitFor(() => expect(screen.getByLabelText('Email')).toHaveFocus())
    expect(request).toHaveBeenCalledOnce()
  })

  it('validates email without sending and formats rate limits from Retry-After', async () => {
    const request = vi.fn(() =>
      HttpResponse.json(problem('rate_limited', 429), {
        status: 429,
        headers: {
          'Content-Type': 'application/problem+json',
          'Retry-After': '120',
        },
      }),
    )
    server.use(http.post(`${base}/auth/password-reset`, request))
    renderApp(['/forgot-password'], {
      routes: [{ path: '/forgot-password', Component: ForgotPasswordPage }],
    })
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Enviar enlace' }))
    expect(await screen.findByText('Completá este campo.')).toBeVisible()
    expect(request).not.toHaveBeenCalled()
    await user.type(screen.getByLabelText('Email'), 'not-an-email')
    await user.click(screen.getByRole('button', { name: 'Enviar enlace' }))
    expect(
      await screen.findByText(
        'Ingresá un email válido, por ejemplo nombre@empresa.com',
      ),
    ).toBeVisible()
    expect(request).not.toHaveBeenCalled()
    await user.clear(screen.getByLabelText('Email'))
    await user.type(screen.getByLabelText('Email'), 'ana@example.test')
    await user.click(screen.getByRole('button', { name: 'Enviar enlace' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Esperá 2 minutos y probá de nuevo.',
    )
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
  })

  // T-F402 (DD-20): an offline request keeps the form available for explicit retry.
  it('offers a retry after a network error and resubmits with the same email', async () => {
    let attempts = 0
    const request = vi.fn(() => {
      attempts += 1
      return attempts === 1
        ? HttpResponse.error()
        : HttpResponse.json(null, { status: 202 })
    })
    server.use(http.post(`${base}/auth/password-reset`, request))
    renderApp(['/forgot-password'], {
      routes: [{ path: '/forgot-password', Component: ForgotPasswordPage }],
    })
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Email'), 'ana@example.test')
    await user.click(screen.getByRole('button', { name: 'Enviar enlace' }))
    expect(
      await screen.findByRole('button', { name: 'Reintentar' }),
    ).toBeVisible()
    expect(screen.getByLabelText('Email')).toHaveValue('ana@example.test')
    await user.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(
      await screen.findByRole('heading', { name: 'Revisá tu email' }),
    ).toBeVisible()
    expect(request).toHaveBeenCalledTimes(2)
  })

  // T-F402 (INV-F01): a current session leaves the public reset screen.
  it('redirects a signed-in visitor to the home page', async () => {
    server.use(
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
    )
    const { router } = renderApp(['/forgot-password'])
    expect(
      await screen.findByRole('heading', { name: 'Hola, Persona de prueba' }),
    ).toBeVisible()
    expect(router.state.location.pathname).toBe('/')
  })
})
