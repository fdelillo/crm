import { render } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { createMemoryRouter, type InitialEntry } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { appRoutes } from '@/app/router'
import { createTestQueryClient } from './queryClient'

export function renderApp(initialEntries: InitialEntry[] = ['/']) {
  const router = createMemoryRouter(appRoutes, { initialEntries })
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
