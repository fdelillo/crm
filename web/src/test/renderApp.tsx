import type { ReactNode } from 'react'
import type { QueryClient } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { createMemoryRouter, type InitialEntry } from 'react-router'
import { createSessionErrorHandlers } from '@/app/sessionErrors'
import { Application } from '@/app/application'
import { appRoutes } from '@/app/router'
import { createTestQueryClient } from './queryClient'

export function renderApp(
  initialEntries: InitialEntry[] = ['/'],
  options: {
    protectedPage?: ReactNode
    configureQueryClient?: (client: QueryClient) => void
  } = {},
) {
  const routes = options.protectedPage ? structuredRoutes() : appRoutes
  function structuredRoutes(): typeof appRoutes {
    const copy = (routes: typeof appRoutes): typeof appRoutes =>
      routes.map((route) => {
        if (route.path === 'settings/users')
          return { path: route.path, element: options.protectedPage }
        if (route.index) return { ...route }
        return {
          ...route,
          ...(route.children ? { children: copy(route.children) } : {}),
        }
      })
    return copy(appRoutes)
  }
  const router = createMemoryRouter(routes, { initialEntries })
  const queryClient = createTestQueryClient({
    onUnauthenticated: () => handlers.onUnauthenticated(),
    onForbidden: () => handlers.onForbidden(),
  })
  const handlers = createSessionErrorHandlers(queryClient, router)
  options.configureQueryClient?.(queryClient)
  return {
    ...render(<Application router={router} queryClient={queryClient} />),
    router,
    queryClient,
  }
}
