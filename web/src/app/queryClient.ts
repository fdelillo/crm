import { QueryClient } from '@tanstack/react-query'
import { isRetryable } from '@/api/errors'

export function createAppQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: true,
        retry: (failureCount, error) => failureCount < 2 && isRetryable(error),
      },
      mutations: { retry: 0, networkMode: 'always' },
    },
  })
}
