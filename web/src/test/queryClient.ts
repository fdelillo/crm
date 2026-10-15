import { QueryClient } from '@tanstack/react-query'

const clients = new Set<QueryClient>()
export function createTestQueryClient() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  clients.add(client)
  return client
}
export function clearTestQueryClients() {
  for (const client of clients) client.clear()
  clients.clear()
}
