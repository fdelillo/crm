import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useNavigate } from 'react-router'
import { api } from '@/api/client'
import { unwrap } from '@/api/errors'
import { queryKeys } from '@/api/queryKeys'
import type { SignupRequest } from '@/api/types'

export function useIndustryTemplates() {
  return useQuery({
    queryKey: queryKeys.industryTemplates,
    queryFn: async ({ signal }) =>
      unwrap(await api.GET('/industry-templates', { signal })),
    staleTime: 3_600_000,
  })
}

export function useSignup() {
  const client = useQueryClient()
  const navigate = useNavigate()
  return useMutation({
    mutationFn: async (body: SignupRequest) =>
      unwrap(await api.POST('/auth/signup', { body })),
    onSuccess: async (session) => {
      await client.cancelQueries()
      client.clear()
      client.setQueryData(queryKeys.session, session)
      toast.success('¡Listo! Tu empresa quedó creada.')
      void navigate('/', { replace: true })
    },
  })
}
