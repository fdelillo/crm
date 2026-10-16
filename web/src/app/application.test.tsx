import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, type RouteObject } from 'react-router'
import { afterEach, expect, it, vi } from 'vitest'
import { http, HttpResponse } from 'msw'
import { toast } from 'sonner'
import { buildSessionInfo, buildTenant } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { appRoutes } from './router'
import { Application, createApplication } from './application'

const base = 'https://localhost:5173/api/v1'
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
    // The boundary's P-F18 effect moves focus to its <h1>; wait for it first.
    const errorHeading = await screen.findByRole('heading', {
      name: 'Algo salió mal al mostrar esta pantalla',
    })
    await waitFor(() => expect(errorHeading).toHaveFocus())
    screen.getByRole('button', { name: 'Actualizar' }).focus()
    await userEvent.keyboard('{Enter}')
    expect(reload).toHaveBeenCalledTimes(2)
  } finally {
    second.view.unmount()
    second.application.dispose()
  }
})
// T-F106 (P-F18): the first screen's chunk fails before RootLayout is mounted.
it('titles and focuses the render-error screen when the first screen chunk fails', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => undefined)
  server.use(
    http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
    http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
  )
  const brokenIndex = (routes: RouteObject[]): RouteObject[] =>
    routes.map((route) =>
      route.index
        ? {
            index: true,
            lazy: async () => {
              throw new Error('Initial screen chunk is unavailable')
            },
          }
        : {
            ...route,
            ...(route.children
              ? { children: brokenIndex(route.children) }
              : {}),
          },
    )
  const application = createApplication({
    createRouter: () =>
      createMemoryRouter(brokenIndex(appRoutes), { initialEntries: ['/'] }),
  })
  const view = render(<Application {...application} />)
  try {
    const heading = await screen.findByRole('heading', {
      name: 'Algo salió mal al mostrar esta pantalla',
    })
    await waitFor(() => expect(heading).toHaveFocus())
    expect(document.title).toBe('Algo salió mal al mostrar esta pantalla · CRM')
  } finally {
    view.unmount()
    application.dispose()
  }
})
afterEach(() => toast.dismiss())
