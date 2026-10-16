import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { unwrap } from '@/api/errors'
import { queryKeys } from '@/api/queryKeys'
import type { Tenant } from '@/api/types'
export function useTenant() {
  return useQuery({
    queryKey: queryKeys.tenant,
    queryFn: async ({ signal }) => unwrap(await api.GET('/tenant', { signal })),
    staleTime: 300_000,
  })
}
export function logoUrl(
  tenant: Pick<Tenant, 'id' | 'updated_at' | 'has_logo'>,
): string | null {
  return tenant.has_logo
    ? `/api/v1/tenant/logo?v=${encodeURIComponent(tenant.id + '-' + tenant.updated_at)}`
    : null
}
