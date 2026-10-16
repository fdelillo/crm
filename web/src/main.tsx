import { StrictMode, lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router/dom'
import { createAppQueryClient } from './app/queryClient'
import { createAppRouter } from './app/router'
import './styles/globals.css'
import { createSessionErrorHandlers } from './app/sessionErrors'

const QueryDevtools = import.meta.env.DEV
  ? lazy(() =>
      import('@tanstack/react-query-devtools').then((module) => ({
        default: module.ReactQueryDevtools,
      })),
    )
  : () => null
const queryClient = createAppQueryClient({
  onUnauthenticated: () => sessionErrors.onUnauthenticated(),
  onForbidden: () => sessionErrors.onForbidden(),
})
const router = createAppRouter()
const sessionErrors = createSessionErrorHandlers(queryClient, router)
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
      <Suspense>
        <QueryDevtools />
      </Suspense>
    </QueryClientProvider>
  </StrictMode>,
)

// ADR-020: the worker caches only the standalone offline page, never app or API data.
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  const register = () => {
    void navigator.serviceWorker
      .register('/sw.js', { scope: '/' })
      .catch(() => undefined)
  }
  if (document.readyState === 'complete') register()
  else window.addEventListener('load', register, { once: true })
}
