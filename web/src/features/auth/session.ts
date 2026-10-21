import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
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

// T-F504 (BR-F14, DD-F28): this is a UX gate; the API remains authoritative.
export function useInviteBlockedReason(): 'email_not_verified' | null {
  return useSession().data?.user.email_verified === false
    ? 'email_not_verified'
    : null
}

export type ResendVerificationOutcome = 'sent' | 'already_verified' | 'unknown'

/** DD-F31: the post-send session read bypasses the cached staleTime result. */
export function useResendEmailVerification() {
  const client = useQueryClient()
  return useMutation<ResendVerificationOutcome, ApiError, void>({
    mutationFn: async () => {
      unwrap<void>(await api.POST('/auth/email-verification/resend'))
      try {
        await client.cancelQueries({ queryKey: queryKeys.session, exact: true })
        const session = await client.fetchQuery<SessionInfo>({
          queryKey: queryKeys.session,
          queryFn: async ({ signal }) =>
            unwrap(await api.GET('/me', { signal })),
          staleTime: 0,
        })
        return session.user.email_verified ? 'already_verified' : 'sent'
      } catch {
        return 'unknown'
      }
    },
    onSuccess: (outcome) => {
      if (outcome === 'already_verified')
        toast('Tu email ya está confirmado. No hace falta otro enlace.')
    },
  })
}
