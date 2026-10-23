import { QueryClient, QueryCache, MutationCache } from '@tanstack/react-query'
import { queryKeys } from '@/api/queryKeys'
import { isApiError, isRetryable, isUnauthenticated } from '@/api/errors'
import { markSessionPresented, sessionLifecycleOf } from '@/lib/sessionExit'
export interface AppQueryClientOptions {
  onUnauthenticated: () => void
  onForbidden: () => void
}
export function createAppQueryClient(options?: AppQueryClientOptions) {
  const onError = (error: unknown) => {
    if (!isApiError(error)) return
    if (isUnauthenticated(error)) options?.onUnauthenticated()
    if (error.status === 403 && error.code === 'forbidden')
      options?.onForbidden()
  }
  const isSession = (query: { queryKey: readonly unknown[] }) =>
    query.queryKey.length === 1 && query.queryKey[0] === queryKeys.session[0]
  const queryCache = new QueryCache({
    onError,
    onSuccess: (data, query) => {
      if (
        isSession(query) &&
        data === null &&
        sessionLifecycleOf(client).presented
      )
        options?.onUnauthenticated()
    },
  })
  const client: QueryClient = new QueryClient({
    queryCache,
    mutationCache: new MutationCache({ onError }),
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: true,
        retry: (failureCount, error) => failureCount < 2 && isRetryable(error),
      },
      mutations: { retry: 0, networkMode: 'always' },
    },
  })
  // Includes sessions seeded by login/signup as well as GET /me (DD-F35). Only the
  // "presented a session" flag is kept; the session itself stays in the query cache.
  queryCache.subscribe((event) => {
    if (isSession(event.query) && event.query.state.data)
      markSessionPresented(client)
  })
  return client
}
