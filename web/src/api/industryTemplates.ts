import { useQuery } from '@tanstack/react-query'
import { api } from './client'
import { unwrap } from './errors'
import { queryKeys } from './queryKeys'

export function useIndustryTemplates() {
  return useQuery({
    queryKey: queryKeys.industryTemplates,
    queryFn: async ({ signal }) =>
      unwrap(await api.GET('/industry-templates', { signal })),
    staleTime: 3_600_000,
  })
}
