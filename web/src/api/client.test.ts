import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { buildSessionInfo, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { api } from './client'
import { unwrap } from './errors'
import type { LoginRequest, SessionInfo } from './types'

const baseUrl = `${window.location.origin}/api/v1`
// T-F005 (ADR-018, S-F2): the real typed client serializes requests and normalizes transport failures.
describe('typed API client', () => {
  it('reads the session from the absolute same-origin URL', async () => {
    const session = buildSessionInfo()
    server.use(
      http.get(`${baseUrl}/me`, ({ request }) => {
        expect(request.url).toBe(`${baseUrl}/me`)
        expect(request.credentials).toBe('same-origin')
        return HttpResponse.json<SessionInfo>(session)
      }),
    )
    expect(unwrap(await api.GET('/me'))).toEqual(session)
  })
  it('serializes a login request as JSON', async () => {
    const body: LoginRequest = {
      email: 'persona@example.test',
      password: 'Fictitious password 2026',
    }
    server.use(
      http.post(`${baseUrl}/auth/login`, async ({ request }) => {
        expect(request.headers.get('Content-Type')).toBe('application/json')
        expect(await request.json()).toEqual(body)
        return HttpResponse.json<SessionInfo>(buildSessionInfo())
      }),
    )
    expect(unwrap(await api.POST('/auth/login', { body }))).toEqual(
      buildSessionInfo(),
    )
  })
  it('normalizes rejected fetch calls without a response', async () => {
    server.use(http.get(`${baseUrl}/me`, () => HttpResponse.error()))
    await expect(api.GET('/me')).rejects.toMatchObject({
      kind: 'network',
      status: null,
    })
  })
  it('unwraps an API problem response', async () => {
    server.use(
      http.get(`${baseUrl}/me`, () =>
        HttpResponse.json(problem('unauthenticated', 401), {
          status: 401,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    await expect(api.GET('/me').then(unwrap)).rejects.toMatchObject({
      kind: 'problem',
      code: 'unauthenticated',
    })
  })
})
