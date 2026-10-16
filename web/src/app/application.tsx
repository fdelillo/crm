import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router/dom'
import { toast } from 'sonner'
import { Toaster } from '@/components/ui/sonner'
import { createAppQueryClient } from './queryClient'
import { createAppRouter } from './router'
import { installPreloadErrorHandler } from './preloadErrors'
import { createSessionErrorHandlers } from './sessionErrors'

export function createApplication({
  createRouter = createAppRouter,
  reload = () => window.location.reload(),
}: {
  createRouter?: typeof createAppRouter
  reload?: () => void
} = {}) {
  // A data router starts lazy imports immediately, before React can mount a layout.
  const removePreloadHandler = installPreloadErrorHandler({
    reload,
    showUpdate: (title, action) =>
      toast.info(title, {
        id: 'application-update',
        duration: Infinity,
        action,
      }),
  })
  const queryClient = createAppQueryClient({
    onUnauthenticated: () => sessionErrors.onUnauthenticated(),
    onForbidden: () => sessionErrors.onForbidden(),
  })
  const router = createRouter()
  const sessionErrors = createSessionErrorHandlers(queryClient, router)
  return {
    router,
    queryClient,
    dispose: () => {
      removePreloadHandler()
      router.dispose()
      queryClient.clear()
    },
  }
}

export function Application({
  router,
  queryClient,
  children,
}: Pick<ReturnType<typeof createApplication>, 'router' | 'queryClient'> & {
  children?: ReactNode
}) {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
      <Toaster theme="light" style={{ zIndex: 200 }} />
      {children}
    </QueryClientProvider>
  )
}
