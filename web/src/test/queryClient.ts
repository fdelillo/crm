import type { QueryClient } from '@tanstack/react-query'
import {
  createAppQueryClient,
  type AppQueryClientOptions,
} from '@/app/queryClient'

const clients = new Set<QueryClient>()
export function createTestQueryClient(options?: AppQueryClientOptions) {
  const client = createAppQueryClient(options)
  const defaults = client.getDefaultOptions()
  client.setDefaultOptions({
    ...defaults,
    queries: { ...defaults.queries, retry: false, gcTime: 0 },
  })
  clients.add(client)
  return client
}
export function clearTestQueryClients() {
  for (const client of clients) client.clear()
  clients.clear()
}
