import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { unwrap, type ApiError } from '@/api/errors'
import { queryKeys } from '@/api/queryKeys'
import type { InvitationRequest, RoleChangeRequest, User } from '@/api/types'

export function useUsers() {
  return useQuery({
    queryKey: queryKeys.users,
    queryFn: async ({ signal }) =>
      unwrap(await api.GET('/users', { signal })).items,
  })
}

export function useInviteUser() {
  const client = useQueryClient()
  return useMutation<
    { user: User; reissued: boolean },
    ApiError,
    InvitationRequest
  >({
    mutationFn: async (body) => {
      const result = await api.POST('/users/invitations', { body })
      return { user: unwrap(result), reissued: result.response.status === 200 }
    },
    onSuccess: async () =>
      client.invalidateQueries({ queryKey: queryKeys.users }),
  })
}

export function useChangeUserRole() {
  const client = useQueryClient()
  return useMutation<
    User,
    ApiError,
    { userId: string; role: RoleChangeRequest['role'] }
  >({
    mutationFn: async ({ userId, role }) =>
      unwrap(
        await api.PUT('/users/{userId}/role', {
          params: { path: { userId } },
          body: { role },
        }),
      ),
    onSuccess: async () =>
      client.invalidateQueries({ queryKey: queryKeys.users }),
  })
}

export function useDeactivateUser() {
  const client = useQueryClient()
  return useMutation<User, ApiError, { userId: string }>({
    mutationFn: async ({ userId }) =>
      unwrap(
        await api.POST('/users/{userId}/deactivate', {
          params: { path: { userId } },
        }),
      ),
    onSuccess: async () =>
      client.invalidateQueries({ queryKey: queryKeys.users }),
  })
}

export function useReactivateUser() {
  const client = useQueryClient()
  return useMutation<User, ApiError, { userId: string }>({
    mutationFn: async ({ userId }) =>
      unwrap(
        await api.POST('/users/{userId}/reactivate', {
          params: { path: { userId } },
        }),
      ),
    onSuccess: async () =>
      client.invalidateQueries({ queryKey: queryKeys.users }),
  })
}
