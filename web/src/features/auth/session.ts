import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { isUnauthenticated, unwrap, type ApiError } from '@/api/errors'
import { queryKeys } from '@/api/queryKeys'
import type { Permission, SessionInfo } from '@/api/types'
export function useSession() {
  return useQuery<SessionInfo | null, ApiError>({
    queryKey: queryKeys.session,
    queryFn: async ({ signal }) => {
      try {
        return unwrap(await api.GET('/me', { signal }))
      } catch (error) {
        if (isUnauthenticated(error)) return null
        throw error
      }
    },
    staleTime: 60_000,
    refetchOnWindowFocus: (query) =>
      query.state.data?.user.email_verified === false ? 'always' : true,
  })
}
export function useCurrentSession() {
  const { data } = useSession()
  if (!data) throw new Error('Current session requires RequireSession')
  return data
}
export function useCan(permission: Permission): boolean {
  return useSession().data?.permissions.includes(permission) ?? false
}
