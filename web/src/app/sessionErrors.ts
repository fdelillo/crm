import type { QueryClient } from '@tanstack/react-query'
import type { Location, NavigateOptions } from 'react-router'
import { toast } from 'sonner'
import { queryKeys } from '@/api/queryKeys'
import { sessionExpiredLoginPath } from '@/lib/sessionExpiredRedirect'
import { hasUnsavedChanges } from '@/lib/unsavedChanges'
export function expireSession(
  client: QueryClient,
  location: Pick<Location, 'pathname' | 'search'>,
  navigate: (to: string, options: NavigateOptions) => unknown,
) {
  if (location.pathname === '/login') return
  const unsavedChangesLost = hasUnsavedChanges()
  client.clear()
  return navigate(sessionExpiredLoginPath(location), {
    replace: true,
    ...(unsavedChangesLost ? { state: { unsavedChangesLost: true } } : {}),
  })
}
export function createSessionErrorHandlers(
  client: QueryClient,
  router: {
    state: {
      location: Location
      navigation: { location?: Location }
    }
    navigate: (to: string, options: NavigateOptions) => Promise<void>
  },
) {
  let redirecting = false
  return {
    onUnauthenticated: () => {
      if (
        redirecting ||
        router.state.location.pathname === '/login' ||
        router.state.navigation.location?.pathname === '/login'
      )
        return
      redirecting = true
      void Promise.resolve(
        expireSession(client, router.state.location, (to, options) =>
          router.navigate(to, options),
        ),
      ).finally(() => {
        redirecting = false
      })
    },
    onForbidden: () => {
      toast.error('No tenés permiso para hacer esto.')
      void client.invalidateQueries({ queryKey: queryKeys.session })
    },
  }
}
