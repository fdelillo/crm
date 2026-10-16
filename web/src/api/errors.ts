import type {
  ErrorCode,
  FieldError,
  FieldErrorCode,
  Problem,
  SuggestedAction,
} from './types'

export type ApiErrorKind = 'problem' | 'network' | 'unexpected'
export interface ApiError extends Error {
  readonly kind: ApiErrorKind
  readonly status: number | null
  readonly code: ErrorCode | null
  readonly problem: Problem | null
  readonly fieldErrors: FieldError[]
  readonly suggestedAction: SuggestedAction | null
  readonly retryAfterSeconds: number | null
  readonly requestId: string | null
  // Date.now() when the error was normalized: anchors Retry-After (P-F18).
  readonly receivedAt: number
}

type ErrorDetails = Omit<ApiError, keyof Error>
class NormalizedApiError extends Error implements ApiError {
  readonly kind: ApiErrorKind
  readonly status: number | null
  readonly code: ErrorCode | null
  readonly problem: Problem | null
  readonly fieldErrors: FieldError[]
  readonly suggestedAction: SuggestedAction | null
  readonly retryAfterSeconds: number | null
  readonly requestId: string | null
  readonly receivedAt: number

  constructor(details: ErrorDetails, cause?: unknown) {
    super('API request failed', { cause })
    this.name = 'ApiError'
    this.kind = details.kind
    this.status = details.status
    this.code = details.code
    this.problem = details.problem
    this.fieldErrors = details.fieldErrors
    this.suggestedAction = details.suggestedAction
    this.retryAfterSeconds = details.retryAfterSeconds
    this.requestId = details.requestId
    this.receivedAt = details.receivedAt
  }
}

const knownCodes = {
  malformed_request: true,
  validation_failed: true,
  unsupported_media_type: true,
  payload_too_large: true,
  unauthenticated: true,
  invalid_credentials: true,
  account_disabled: true,
  forbidden: true,
  not_found: true,
  method_not_allowed: true,
  email_already_registered: true,
  email_taken: true,
  email_not_verified: true,
  last_admin: true,
  invalid_state: true,
  token_invalid: true,
  login_locked: true,
  rate_limited: true,
  service_unavailable: true,
  internal: true,
} satisfies Record<ErrorCode, true>
const fieldCodes = {
  required: true,
  invalid_format: true,
  too_short: true,
  too_long: true,
  invalid_value: true,
  invalid_tax_id: true,
  same_as_email: true,
  unknown_template: true,
  invalid_timezone: true,
} satisfies Record<FieldErrorCode, true>

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
function isProblem(value: unknown): value is Problem {
  return (
    isRecord(value) &&
    typeof value.type === 'string' &&
    typeof value.title === 'string' &&
    Number.isInteger(value.status) &&
    Number(value.status) >= 400 &&
    Number(value.status) <= 599 &&
    typeof value.code === 'string'
  )
}
function isFieldError(value: unknown): value is FieldError {
  return (
    isRecord(value) &&
    typeof value.field === 'string' &&
    typeof value.code === 'string' &&
    Object.hasOwn(fieldCodes, value.code)
  )
}
function retryAfter(response: Response): number | null {
  if (response.status !== 429 && response.status !== 503) return null
  const value = response.headers.get('Retry-After')
  if (!value || !/^\d+$/.test(value)) return null
  const seconds = Number(value)
  return Number.isSafeInteger(seconds) ? seconds : null
}
export function toApiError(input: {
  response?: Response
  body?: unknown
  cause?: unknown
}): ApiError {
  const { response, body, cause } = input
  const receivedAt = Date.now()
  const shared = {
    receivedAt,
    code: null,
    problem: null,
    fieldErrors: [],
    suggestedAction: null,
    retryAfterSeconds: null,
    requestId: null,
  }
  if (!response)
    return new NormalizedApiError(
      { ...shared, kind: 'network', status: null },
      cause,
    )
  if (
    response.headers
      .get('Content-Type')
      ?.split(';')[0]
      ?.trim()
      .toLowerCase() !== 'application/problem+json' ||
    !isProblem(body)
  ) {
    return new NormalizedApiError(
      {
        ...shared,
        kind: 'unexpected',
        status: response.status,
        retryAfterSeconds: retryAfter(response),
      },
      cause,
    )
  }
  const code = Object.hasOwn(knownCodes, body.code) ? body.code : null
  const errors =
    'errors' in body && Array.isArray(body.errors) ? body.errors : []
  return new NormalizedApiError(
    {
      receivedAt,
      kind: 'problem',
      status: response.status,
      code,
      problem: body,
      fieldErrors:
        code === 'validation_failed' ? errors.filter(isFieldError) : [],
      suggestedAction:
        body.suggested_action === 'password_reset' ? 'password_reset' : null,
      retryAfterSeconds: retryAfter(response),
      requestId: typeof body.instance === 'string' ? body.instance : null,
    },
    cause,
  )
}
export function isApiError(value: unknown): value is ApiError {
  return value instanceof NormalizedApiError
}
export function unwrap<T>(result: {
  data?: T
  error?: unknown
  response: Response
}): T {
  if (!result.response.ok)
    throw toApiError({ response: result.response, body: result.error })
  return result.data as T
}
export function isRetryable(error: unknown): boolean {
  return isApiError(error) && (error.kind === 'network' || error.status === 503)
}

/** Shared by the session query and the global error handler (INV-F02). */
export function isUnauthenticated(error: unknown): boolean {
  return (
    isApiError(error) &&
    error.status === 401 &&
    error.code === 'unauthenticated'
  )
}
