import createClient from 'openapi-fetch'
import { toApiError } from './errors'
import type { paths } from './schema'

export const api = createClient<paths>({
  baseUrl: `${window.location.origin}/api/v1`,
  credentials: 'same-origin',
  async fetch(request) {
    try {
      return await fetch(request)
    } catch (cause) {
      throw toApiError({ cause })
    }
  },
})
