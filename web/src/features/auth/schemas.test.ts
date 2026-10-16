import { describe, expect, it } from 'vitest'
import { signupSchema } from './schemas'
import type { SignupRequest } from '@/api/types'

const validSignup: SignupRequest = {
  name: 'Ana Pérez',
  email: 'ana@example.test',
  password: 'contraseña-segura',
  company_name: 'Aberturas Norte',
  base_currency: 'ARS',
  industry_template_code: 'generic',
}

describe('signupSchema', () => {
  it('normalizes email and trims text fields', () => {
    expect(
      signupSchema.parse({
        ...validSignup,
        name: ' Ana Pérez ',
        email: ' ANA@Example.Test ',
        company_name: ' Aberturas Norte ',
      }),
    ).toEqual({
      ...validSignup,
      name: 'Ana Pérez',
      email: 'ana@example.test',
      company_name: 'Aberturas Norte',
    })
  })

  it.each([
    ['name', '', 'required'],
    ['name', 'a'.repeat(121), 'too_long'],
    ['company_name', '', 'required'],
    ['company_name', 'a'.repeat(121), 'too_long'],
    ['email', 'ana@', 'invalid_format'],
    ['password', '123456789', 'too_short'],
    ['password', 'p'.repeat(129), 'too_long'],
    ['industry_template_code', '', 'required'],
  ] as const)('rejects %s with the expected validation code', (field, value, code) => {
    const result = signupSchema.safeParse({ ...validSignup, [field]: value })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0]?.message).toBe(code)
    }
  })

  it.each([10, 128])('accepts a password of %i characters', (length) => {
    expect(
      signupSchema.safeParse({ ...validSignup, password: 'p'.repeat(length) })
        .success,
    ).toBe(true)
  })

  it('rejects a password equal to the email regardless of case', () => {
    const result = signupSchema.safeParse({
      ...validSignup,
      email: 'ANA@example.test',
      password: 'ana@EXAMPLE.test',
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0]?.message).toBe('same_as_email')
    }
  })
})
