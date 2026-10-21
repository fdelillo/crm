import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from './client'
import { unwrap } from './errors'
import { queryKeys } from './queryKeys'
import type { Tenant } from './types'

// Shared with the logo mutations, which re-read the tenant to settle an uncertain failure (DD-F26).
export const tenantQuery = queryOptions({
  queryKey: queryKeys.tenant,
  queryFn: async ({ signal }) => unwrap(await api.GET('/tenant', { signal })),
  staleTime: 300_000,
})

export function useTenant({ enabled = true }: { enabled?: boolean } = {}) {
  return useQuery({ ...tenantQuery, enabled })
}

export function logoUrl(
  tenant: Pick<Tenant, 'id' | 'updated_at' | 'has_logo'>,
): string | null {
  return tenant.has_logo
    ? `/api/v1/tenant/logo?v=${encodeURIComponent(tenant.id + '-' + tenant.updated_at)}`
    : null
}
