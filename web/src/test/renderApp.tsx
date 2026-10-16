import type { ReactNode } from 'react'
import { render } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { createMemoryRouter, type InitialEntry } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { appRoutes } from '@/app/router'
import { createTestQueryClient } from './queryClient'

export function renderApp(
  initialEntries: InitialEntry[] = ['/'],
  options: { protectedPage?: ReactNode } = {},
) {
  const routes = options.protectedPage ? structuredRoutes() : appRoutes
  function structuredRoutes(): typeof appRoutes {
    const copy = (routes: typeof appRoutes): typeof appRoutes =>
      routes.map((route) =>
        route.path === 'settings/users'
          ? { path: route.path, element: options.protectedPage }
          : {
              ...route,
              ...(route.children ? { children: copy(route.children) } : {}),
            },
      )
    return copy(appRoutes)
  }
  const router = createMemoryRouter(routes, { initialEntries })
  const queryClient = createTestQueryClient()
  return {
    ...render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    ),
    router,
    queryClient,
  }
}
