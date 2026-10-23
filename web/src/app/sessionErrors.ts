import type { QueryClient } from '@tanstack/react-query'
import type { Location } from 'react-router'
import { toast } from 'sonner'
import { queryKeys } from '@/api/queryKeys'
import {
  attachSessionExitRouter,
  exitSession,
  type SessionExitRouter,
} from '@/lib/sessionExit'

export function createSessionErrorHandlers(
  client: QueryClient,
  router: SessionExitRouter & {
    state: {
      location: Location
      navigation: { location?: Location }
    }
  },
) {
  attachSessionExitRouter(client, router)
  return {
    onUnauthenticated: () => {
      if (
        router.state.location.pathname === '/login' ||
        router.state.navigation.location?.pathname === '/login'
      )
        return
      // A 401 or a null session during an exit in progress changes nothing (DD-F35).
      exitSession(client, 'session_expired')
    },
    onForbidden: () => {
      toast.error('No tenés permiso para hacer esto.')
      void client.invalidateQueries({ queryKey: queryKeys.session })
    },
  }
}
