import { describe, expect, it } from 'vitest'
import { problem } from '@/test/fixtures'
import { isApiError, isRetryable, toApiError, unwrap } from './errors'
import type { ErrorCode, FieldError, ValidationProblem } from './types'

function response(status: number, headers: HeadersInit = {}) {
  const mergedHeaders = new Headers(headers)
  mergedHeaders.set('Content-Type', 'application/problem+json')
  return new Response(null, { status, headers: mergedHeaders })
}
// T-F005: normalize the API error contract without React or browser state.
describe('API errors', () => {
  it('preserves a registration conflict and its recovery action', () => {
    const error = toApiError({
      response: response(409),
      body: problem('email_already_registered', 409, {
        suggested_action: 'password_reset',
      }),
    })
    expect(error).toMatchObject({
      kind: 'problem',
      status: 409,
      code: 'email_already_registered',
      suggestedAction: 'password_reset',
    })
    expect(isApiError(error)).toBe(true)
    expect(isApiError(new Error('ordinary'))).toBe(false)
  })
  it('keeps both field errors in order', () => {
    const errors: FieldError[] = [
      { field: 'name', code: 'required' },
      { field: 'email', code: 'invalid_format' },
    ]
    const body: ValidationProblem = {
      ...problem('validation_failed', 422),
      errors,
    }
    expect(toApiError({ response: response(422), body }).fieldErrors).toEqual(
      errors,
    )
  })
  it.each([
    [429, 'login_locked', '900', 900],
    [429, 'rate_limited', undefined, null],
    [503, 'service_unavailable', '2', 2],
  ] as const)(
    'reads Retry-After for status %s and code %s',
    (status, code, retry, expected) => {
      const headers = new Headers()
      if (retry) headers.set('Retry-After', retry)
      const error = toApiError({
        response: response(status, headers),
        body: problem(code, status),
      })
      expect(error.retryAfterSeconds).toBe(expected)
    },
  )
  it.each(['', 'invalid', '-1', '2.5', 'Infinity'])(
    'does not trust invalid Retry-After %s',
    (value) => {
      expect(
        toApiError({
          response: response(429, { 'Retry-After': value }),
          body: problem('rate_limited', 429),
        }).retryAfterSeconds,
      ).toBeNull()
    },
  )
  it.each([
    [405, 'method_not_allowed'],
    [403, 'forbidden'],
  ] as const)('preserves problem status %s and code %s', (status, code) => {
    expect(
      toApiError({
        response: response(status, { Allow: 'GET' }),
        body: problem(code, status),
      }),
    ).toMatchObject({ kind: 'problem', status, code })
  })
  it('exposes the request reference for support', () => {
    expect(
      toApiError({
        response: response(500),
        body: problem('internal', 500, { instance: 'request-example' }),
      }).requestId,
    ).toBe('request-example')
  })
  it('handles a proxy HTML response', () => {
    expect(
      toApiError({
        response: new Response('<h1>Bad gateway</h1>', {
          status: 502,
          headers: { 'Content-Type': 'text/html' },
        }),
        body: '<h1>Bad gateway</h1>',
      }),
    ).toMatchObject({
      kind: 'unexpected',
      status: 502,
      code: null,
      problem: null,
    })
  })
  it('retains a future problem code while exposing no known code', () => {
    const body = { ...problem('internal', 500), code: 'future_code' }
    const error = toApiError({ response: response(500), body })
    expect(error).toMatchObject({ kind: 'problem', code: null, problem: body })
  })
  it('turns transport failures into network errors', () => {
    expect(
      toApiError({ cause: new TypeError('Failed to fetch') }),
    ).toMatchObject({ kind: 'network', status: null, code: null })
  })
  it('unwraps data and empty successful responses and throws normalized errors', () => {
    const data = { name: 'Example' }
    expect(unwrap({ data, response: new Response() })).toBe(data)
    expect(
      unwrap({ response: new Response(null, { status: 204 }) }),
    ).toBeUndefined()
    expect(() =>
      unwrap({ error: problem('forbidden', 403), response: response(403) }),
    ).toThrow(expect.objectContaining({ kind: 'problem', code: 'forbidden' }))
  })
  it.each([
    [503, 'service_unavailable', true],
    [500, 'internal', false],
    [429, 'rate_limited', false],
    [400, 'malformed_request', false],
    [401, 'unauthenticated', false],
    [403, 'forbidden', false],
    [409, 'invalid_state', false],
  ] satisfies [number, ErrorCode, boolean][])(
    'retries status %s only if temporary',
    (status, code, retryable) => {
      expect(
        isRetryable(
          toApiError({
            response: response(status),
            body: problem(code, status),
          }),
        ),
      ).toBe(retryable)
    },
  )
  it('retries transport failures but not arbitrary errors', () => {
    expect(isRetryable(toApiError({ cause: new TypeError('Offline') }))).toBe(
      true,
    )
    expect(isRetryable(new Error('bug'))).toBe(false)
  })
})
