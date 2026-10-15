import { expectTypeOf, test } from 'vitest'
import type { paths as Paths001 } from './generated/001'
import type { paths as ExternalPaths } from '../../test-fixtures/openapi/generated'
import type { AssertDisjoint } from './schema'
import type {
  ErrorCode,
  FieldErrorCode,
  SessionInfo,
  TenantUpdate,
  User,
} from './types'

// T-F004 (INV-F05, S-F1): generated types preserve the canonical contract and external references.
test('preserves response, request and shared error types', () => {
  expectTypeOf<
    Paths001['/me']['get']['responses'][200]['content']['application/json']
  >().toEqualTypeOf<SessionInfo>()
  expectTypeOf<User['name']>().toEqualTypeOf<string | null>()
  expectTypeOf<TenantUpdate['name']>().toEqualTypeOf<string | undefined>()
  expectTypeOf<TenantUpdate['timezone']>().toEqualTypeOf<string | undefined>()
  expectTypeOf<Extract<keyof TenantUpdate, 'base_currency'>>().toBeNever()
  expectTypeOf<
    | 'email_already_registered'
    | 'email_taken'
    | 'last_admin'
    | 'token_invalid'
    | 'payload_too_large'
    | 'unsupported_media_type'
    | 'method_not_allowed'
    | 'email_not_verified'
  >().toExtend<ErrorCode>()
  expectTypeOf<'required' | 'invalid_value'>().toExtend<FieldErrorCode>()
  expectTypeOf<
    Paths001['/users/invitations']['post']['responses']
  >().toHaveProperty(429)
  expectTypeOf<
    Paths001['/users/{userId}/reactivate']['post']['responses']
  >().toHaveProperty(429)
  expectTypeOf<Paths001['/auth/signup']['post']['responses']>().toHaveProperty(
    413,
  )
  expectTypeOf<
    NonNullable<Paths001['/tenant/logo']['get']['parameters']['query']>['v']
  >().toEqualTypeOf<string | undefined>()
  expectTypeOf<
    Paths001['/tenant/logo']['put']['requestBody']['content']['multipart/form-data']['file']
  >().toEqualTypeOf<string>()
  expectTypeOf<
    ExternalPaths['/external-problem']['get']['responses'][400]['content']['application/problem+json']['code']
  >().toEqualTypeOf<ErrorCode>()
})

// T-F004 (DD-17): combining specifications must never silently merge repeated routes.
test('rejects overlapping route keys', () => {
  const disjoint: AssertDisjoint<keyof Paths001, keyof ExternalPaths> = true
  expectTypeOf(disjoint).toEqualTypeOf<true>()
  // @ts-expect-error Duplicate paths cannot satisfy the disjoint assertion.
  const overlapping: AssertDisjoint<keyof Paths001, '/me'> = true
  expectTypeOf(overlapping).toBeNever()
})
