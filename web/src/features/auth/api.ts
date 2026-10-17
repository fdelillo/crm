import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useLocation, useNavigate } from 'react-router'
import { api } from '@/api/client'
import { isApiError, unwrap, type ApiError } from '@/api/errors'
import { queryKeys } from '@/api/queryKeys'
import { messageForError } from '@/api/errorMessages'
export { useIndustryTemplates } from '@/api/industryTemplates'
import type { LoginRequest, SignupRequest } from '@/api/types'
import { safeNextPath } from '@/lib/safeNextPath'

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

export function useLogin() {
  const client = useQueryClient()
  const navigate = useNavigate()
  const location = useLocation()
  return useMutation({
    mutationFn: async (body: LoginRequest) =>
      unwrap(await api.POST('/auth/login', { body })),
    onSuccess: async (session) => {
      await client.cancelQueries()
      client.clear()
      client.setQueryData(queryKeys.session, session)
      const next = new URLSearchParams(location.search).get('next')
      void navigate(safeNextPath(next), { replace: true })
    },
  })
}

export function useLogout() {
  const client = useQueryClient()
  const navigate = useNavigate()
  return useMutation<void, ApiError, void>({
    mutationFn: async () => {
      unwrap<void>(await api.POST('/auth/logout'))
    },
    onSuccess: async () => {
      await client.cancelQueries()
      client.clear()
      void navigate('/login?reason=logged_out', { replace: true })
    },
    onError: (error) => {
      if (isApiError(error) && error.kind === 'network') {
        toast.error('No pudimos cerrar la sesión. Revisá tu conexión.')
        return
      }
      const message = messageForError(error, 'generic')
      toast.error(message.title, {
        ...(message.description ? { description: message.description } : {}),
      })
    },
  })
}
