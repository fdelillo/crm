import {
  useMutation,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query'
import { api } from '@/api/client'
import { isApiError, unwrap, type ApiError } from '@/api/errors'
import { queryKeys } from '@/api/queryKeys'
import { logoVersion, tenantQuery } from '@/api/tenant'
import type { Tenant, TenantUpdate } from '@/api/types'

export { logoUrl, useTenant } from '@/api/tenant'

const refreshSession = (client: QueryClient) =>
  client.invalidateQueries({ queryKey: queryKeys.session, exact: true })

export function useUpdateTenant() {
  const client = useQueryClient()
  return useMutation<Tenant, ApiError, TenantUpdate>({
    mutationFn: async (body) => unwrap(await api.PATCH('/tenant', { body })),
    // INV-F04: the response replaces the cached tenant; the session carries the name (TenantSummary).
    onSuccess: (tenant) => {
      client.setQueryData(queryKeys.tenant, tenant)
      void refreshSession(client)
    },
  })
}

/** PUT /tenant/logo with a single multipart part named `file` (DD-31). */
export async function putTenantLogo(file: File): Promise<Tenant> {
  return unwrap(
    await api.PUT('/tenant/logo', {
      // The generated type models the binary part as a string; the serializer sends the File.
      body: { file: file as unknown as string },
      bodySerializer: (body) => {
        const form = new FormData()
        form.append('file', body.file as unknown as Blob)
        return form
      },
    }),
  )
}

// DD-42: after a 5xx or a network error the server may have committed anyway.
function isUncertain(error: unknown) {
  return (
    isApiError(error) &&
    (error.kind === 'network' || (error.status !== null && error.status >= 500))
  )
}

// DD-F26: read the tenant again ignoring staleTime; null if even that fails.
async function readTenantAgain(client: QueryClient): Promise<Tenant | null> {
  void refreshSession(client)
  try {
    return await client.fetchQuery({ ...tenantQuery, staleTime: 0 })
  } catch {
    return null
  }
}

// DD-F26 (P-F21): updated_at also changes with a PATCH from another session, so it does not
// prove the new logo was saved. The server stores the bytes untouched (INV-24), so the logo is
// ours only if the current one has exactly the bytes we sent. Any failure here means "not proven".
async function currentLogoHasBytesOf(
  sent: File,
  current: Tenant,
): Promise<boolean> {
  try {
    const { data, error } = await api.GET('/tenant/logo', {
      params: { query: { v: logoVersion(current) } },
      parseAs: 'blob',
      cache: 'no-store',
    })
    if (error !== undefined || !data) return false
    const stored = data as unknown as Blob
    if (stored.size !== sent.size) return false
    const [a, b] = await Promise.all([stored.arrayBuffer(), sent.arrayBuffer()])
    const left = new Uint8Array(a)
    const right = new Uint8Array(b)
    return left.every((byte, index) => byte === right[index])
  } catch {
    return false
  }
}

export function useUploadLogo() {
  const client = useQueryClient()
  return useMutation<Tenant, ApiError, File>({
    mutationFn: async (file) => {
      const before = client.getQueryData<Tenant>(queryKeys.tenant)
      try {
        return await putTenantLogo(file)
      } catch (error) {
        if (!isUncertain(error)) throw error
        const current = await readTenantAgain(client)
        // A new logo means has_logo, an updated_at different from the one we started with,
        // and the bytes we sent as the current logo.
        if (
          current?.has_logo &&
          current.updated_at !== before?.updated_at &&
          (await currentLogoHasBytesOf(file, current))
        )
          return current
        throw error
      }
    },
    onSuccess: (tenant) => {
      client.setQueryData(queryKeys.tenant, tenant)
      void refreshSession(client)
    },
  })
}

export function useDeleteLogo() {
  const client = useQueryClient()
  return useMutation<void, ApiError, void>({
    mutationFn: async () => {
      try {
        unwrap<void>(await api.DELETE('/tenant/logo'))
      } catch (error) {
        if (!isUncertain(error)) throw error
        const current = await readTenantAgain(client)
        if (current && !current.has_logo) return
        throw error
      }
    },
    onSuccess: async () => {
      void refreshSession(client)
      await client.invalidateQueries({
        queryKey: queryKeys.tenant,
        exact: true,
      })
    },
  })
}
