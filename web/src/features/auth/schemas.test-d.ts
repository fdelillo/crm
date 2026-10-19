import { describe, expectTypeOf, it } from 'vitest'
import type { SignupRequest } from '@/api/types'
import { signupSchema } from './schemas'
import type { z } from 'zod'

describe('signup request type', () => {
  it('matches the generated OpenAPI request type', () => {
    expectTypeOf<z.output<typeof signupSchema>>().toEqualTypeOf<SignupRequest>()
  })
})
