import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, type RouteObject } from 'react-router'
import { afterEach, expect, it, vi } from 'vitest'
import { toast } from 'sonner'
import { appRoutes } from './router'
import { Application, createApplication } from './application'

// T-F106/T-F107 (review: initial lazy chunk failure): exercise startup before any layout mounts.
it('reloads one failed initial chunk and exposes update after the next failed startup', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => undefined)
  const reload = vi.fn()
  const errors: Event[] = []
  const missingLoginChunk = (routes: RouteObject[]): RouteObject[] =>
    routes.map((route) => {
      if (route.path === 'login')
        return {
          path: route.path,
          lazy: async () => {
            const event = new Event('vite:preloadError', { cancelable: true })
            errors.push(event)
            window.dispatchEvent(event)
            throw new Error('Initial login chunk is unavailable')
          },
        }
      if (route.index) return { ...route }
      return {
        ...route,
        ...(route.children
          ? { children: missingLoginChunk(route.children) }
          : {}),
      }
    })
  function start() {
    const application = createApplication({
      createRouter: () =>
        createMemoryRouter(missingLoginChunk(appRoutes), {
          initialEntries: ['/login'],
        }),
      reload,
    })
    const view = render(<Application {...application} />)
    return { application, view }
  }
  const first = start()
  await waitFor(() => expect(reload).toHaveBeenCalledOnce())
  expect(errors[0]?.defaultPrevented).toBe(true)
  expect(sessionStorage.getItem('crm.preloadReloaded')).toBe('true')
  await act(async () => {
    first.view.unmount()
    first.application.dispose()
  })
  const second = start()
  try {
    expect(
      await screen.findByText('Hay una versión nueva de la app'),
    ).toBeVisible()
    expect(
      await screen.findByRole('button', { name: 'Actualizar' }),
    ).toBeVisible()
    expect(errors[1]?.defaultPrevented).toBe(true)
    expect(reload).toHaveBeenCalledOnce()
    screen.getByRole('button', { name: 'Actualizar' }).focus()
    await userEvent.keyboard('{Enter}')
    expect(reload).toHaveBeenCalledTimes(2)
  } finally {
    second.view.unmount()
    second.application.dispose()
  }
})
afterEach(() => toast.dismiss())
