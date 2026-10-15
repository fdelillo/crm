import { StrictMode, lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router/dom'
import { createAppQueryClient } from './app/queryClient'
import { createAppRouter } from './app/router'
import './styles/globals.css'

const QueryDevtools = import.meta.env.DEV
  ? lazy(() =>
      import('@tanstack/react-query-devtools').then((module) => ({
        default: module.ReactQueryDevtools,
      })),
    )
  : () => null
const queryClient = createAppQueryClient()
const router = createAppRouter()
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
