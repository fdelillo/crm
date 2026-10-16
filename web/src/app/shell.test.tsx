import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  buildSessionInfo,
  buildCurrentUser,
  buildTenant,
} from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { installPreloadErrorHandler } from './preloadErrors'
import { FormAlert } from '@/components/FormAlert'
import { ErrorState } from '@/components/ErrorState'
import { EmptyState } from '@/components/EmptyState'
const base = 'https://localhost:5173/api/v1'
beforeEach(() =>
  server.use(
    http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
    http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
  ),
)
// T-F106 (BR-F04, INV-F10, NFR-F06): shell behavior is accessible via names, roles and focus.
describe('application shell', () => {
  it('shows named navigation and administrator settings', async () => {
    const { router } = renderApp(['/'])
    expect(await screen.findByRole('link', { name: 'Inicio' })).toBeVisible()
    await userEvent.click(screen.getByRole('link', { name: 'Ajustes' }))
    expect(
      await screen.findByRole('heading', { name: 'Ajustes' }),
    ).toBeVisible()
    expect(screen.getByText('Persona de prueba')).toBeVisible()
    expect(screen.getByText('persona@example.test')).toBeVisible()
    expect(screen.getByText('Administrador')).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Datos de la empresa' }),
    ).toHaveAttribute('href', '/settings/company')
    expect(screen.getByRole('link', { name: 'Usuarios' })).toHaveAttribute(
      'href',
      '/settings/users',
    )
    expect(router.state.location.pathname).toBe('/settings')
    expect(screen.getByRole('link', { name: 'Ajustes' })).toHaveAttribute(
      'aria-current',
      'page',
    )
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Ajustes' })).toHaveFocus(),
    )
    expect(document.title).toBe('Ajustes · Empresa de prueba')
  })
  it('shows only personal settings to an operator', async () => {
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            permissions: [],
            user: buildCurrentUser({ role: 'operator' }),
          }),
        ),
      ),
    )
    renderApp(['/settings'])
    await screen.findByRole('heading', { name: 'Ajustes' })
    expect(
      screen.queryByRole('heading', { name: 'Tu empresa' }),
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('link', { name: 'Usuarios' }),
    ).not.toBeInTheDocument()
    expect(screen.getByText('Operador')).toBeVisible()
  })
  it('puts the skip link first in keyboard order and focuses main', async () => {
    renderApp(['/'])
    await screen.findByRole('heading', { name: 'Panel inicial' })
    const user = userEvent.setup()
    await user.tab()
    expect(
      screen.getByRole('link', { name: 'Saltar al contenido' }),
    ).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(screen.getByRole('main')).toHaveFocus()
  })
  it('uses the session name while tenant loads, then its fresh name and initials', async () => {
    let resolve!: () => void
    const pending = new Promise<void>((done) => {
      resolve = done
    })
    server.use(
      http.get(`${base}/tenant`, async () => {
        await pending
        return HttpResponse.json(buildTenant({ name: 'Empresa renovada' }))
      }),
    )
    renderApp(['/'])
    expect(await screen.findByText('Empresa de prueba')).toBeVisible()
    await act(async () => resolve())
    expect(await screen.findByText('Empresa renovada')).toBeVisible()
    expect(screen.getByText('ER')).toBeVisible()
  })
  it('announces connectivity changes', async () => {
    renderApp(['/'])
    await screen.findByRole('heading', { name: 'Panel inicial' })
    Object.defineProperty(navigator, 'onLine', {
      configurable: true,
      value: false,
    })
    act(() => window.dispatchEvent(new Event('offline')))
    expect(
      await screen.findByRole('status', { name: 'Sin conexión' }),
    ).toHaveTextContent('Sin conexión. Revisá tu internet.')
    Object.defineProperty(navigator, 'onLine', {
      configurable: true,
      value: true,
    })
    act(() => window.dispatchEvent(new Event('online')))
    expect(
      screen.queryByRole('status', { name: 'Sin conexión' }),
    ).not.toBeInTheDocument()
  })
  it('shows recovery actions after a render error', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    function BrokenPage(): never {
      throw new Error('Test rendering failure')
    }
    renderApp(['/settings/users'], { protectedPage: <BrokenPage /> })
    expect(
      await screen.findByRole('heading', {
        name: 'Algo salió mal al mostrar esta pantalla',
      }),
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Recargar' })).toBeVisible()
    expect(screen.getByRole('link', { name: 'Ir al inicio' })).toHaveAttribute(
      'href',
      '/',
    )
  })
  // T-F106 (P-F18): the root boundary replaces RootLayout, so it sets its own title and focus.
  it('titles and focuses the render-error screen after navigating from Settings', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    function BrokenPage(): never {
      throw new Error('Test rendering failure')
    }
    renderApp(['/settings'], { protectedPage: <BrokenPage /> })
    await screen.findByRole('heading', { name: 'Ajustes' })
    expect(document.title).toBe('Ajustes · Empresa de prueba')
    await userEvent.click(screen.getByRole('link', { name: 'Usuarios' }))
    const heading = await screen.findByRole('heading', {
      name: 'Algo salió mal al mostrar esta pantalla',
    })
    await waitFor(() => expect(heading).toHaveFocus())
    expect(document.title).toBe('Algo salió mal al mostrar esta pantalla · CRM')
  })
  // T-F106 (P-F18, ui.md §13): without a cached session every title ends in "· CRM".
  it('ends the title in "· CRM" on a screen without a session', async () => {
    renderApp(['/ruta-inexistente'])
    await screen.findByRole('heading', { name: 'No encontramos esta página.' })
    await waitFor(() =>
      expect(document.title).toBe('No encontramos esta página. · CRM'),
    )
  })
  // T-F106 (INV-F10, review): two routes that render the same screen reuse the <h1> node.
  it('moves focus to the heading when navigating between routes that reuse the same heading node', async () => {
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          buildSessionInfo({
            permissions: [],
            user: buildCurrentUser({ role: 'operator' }),
          }),
        ),
      ),
    )
    const { router } = renderApp(['/settings/users'])
    const heading = await screen.findByRole('heading', { level: 1 })
    const link = screen.getByRole('link', { name: 'Inicio' })
    link.focus()
    await act(() => router.navigate('/settings/company'))
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1 })).toHaveFocus(),
    )
    expect(screen.getByRole('heading', { level: 1 })).toBe(heading)
    // The mark was consumed: later DOM changes must not steal the focus again.
    link.focus()
    await act(async () => {
      document.body.appendChild(document.createElement('div'))
      await new Promise((r) => setTimeout(r, 20))
    })
    expect(link).toHaveFocus()
  })
  it('recovers a missing chunk once and then offers an explicit update', async () => {
    const reload = vi.fn()
    const showUpdate = vi.fn()
    const remove = installPreloadErrorHandler({ reload, showUpdate })
    const first = new Event('vite:preloadError', { cancelable: true })
    window.dispatchEvent(first)
    expect(reload).toHaveBeenCalledOnce()
    expect(first.defaultPrevented).toBe(true)
    expect(sessionStorage.getItem('crm.preloadReloaded')).toBe('true')
    window.dispatchEvent(new Event('vite:preloadError', { cancelable: true }))
    expect(reload).toHaveBeenCalledOnce()
    expect(showUpdate).toHaveBeenCalledWith(
      'Hay una versión nueva de la app',
      expect.objectContaining({ label: 'Actualizar' }),
    )
    showUpdate.mock.calls[0]![1].onClick()
    expect(reload).toHaveBeenCalledTimes(2)
    remove()
  })
  it('provides a way home from an unknown route outside the shell', async () => {
    renderApp(['/missing'])
    expect(
      await screen.findByRole('heading', {
        name: 'No encontramos esta página.',
      }),
    ).toBeVisible()
    expect(screen.getByRole('link', { name: 'Ir al inicio' })).toHaveAttribute(
      'href',
      '/',
    )
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
  })
})
// T-F106: shared presentation communicates states and exposes keyboard actions.
it('exposes status, error and empty-state actions', async () => {
  const action = vi.fn()
  const { unmount } = render(
    <FormAlert
      variant="warning"
      title="Aviso"
      description="Detalle"
      action={{ label: 'Continuar', onClick: action }}
    />,
  )
  expect(screen.getByRole('status')).toHaveTextContent('Detalle')
  await userEvent.click(screen.getByRole('button', { name: 'Continuar' }))
  expect(action).toHaveBeenCalledOnce()
  unmount()
  const error = render(
    <ErrorState
      message={{
        title: 'Fallo',
        description: 'Detalle',
        requestId: 'reference',
        retryable: true,
      }}
      onRetry={action}
    />,
  )
  expect(screen.getByRole('alert')).toHaveTextContent('Código: reference')
  expect(screen.getByRole('button', { name: 'Reintentar' })).toBeVisible()
  error.unmount()
  render(
    <EmptyState
      title="Sin datos"
      description="Creá uno"
      action={{ label: 'Crear', onClick: action }}
    />,
  )
  expect(screen.getByRole('heading', { name: 'Sin datos' })).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: 'Crear' }))
  expect(action).toHaveBeenCalledTimes(2)
})
