import { describe, expect, it } from 'vitest'
import {
  buildTenantUpdate,
  companyFields,
  companySchema,
  type CompanyFormValues,
} from './schemas'

const values: CompanyFormValues = {
  name: 'Mi Empresa',
  legal_name: 'ACME S.A.',
  tax_id: '30-12345678-1',
  address: '',
  phone: '',
  email: '',
  timezone: 'America/Argentina/Cordoba',
}

// T-F602 (INV-F15, DD-43, ui.md §13.11)
describe('buildTenantUpdate', () => {
  it('includes only the dirty fields', () => {
    expect(buildTenantUpdate(values, { legal_name: true })).toEqual({
      legal_name: 'ACME S.A.',
    })
    expect(buildTenantUpdate(values, {})).toEqual({})
  })
  it('sends null for a cleared optional field', () => {
    expect(buildTenantUpdate(values, { address: true, tax_id: true })).toEqual({
      address: null,
      tax_id: '30-12345678-1',
    })
  })
  it('never sends null for name or timezone and never base_currency, whatever is dirty', () => {
    const blank = Object.fromEntries(
      companyFields.map((field) => [field, '']),
    ) as CompanyFormValues
    for (let mask = 0; mask < 2 ** companyFields.length; mask++) {
      const dirty = Object.fromEntries(
        companyFields.map((field, index) => [
          field,
          Boolean(mask & (1 << index)),
        ]),
      )
      for (const source of [values, blank]) {
        const update = buildTenantUpdate(source, dirty)
        expect(update.name).not.toBeNull()
        expect(update.timezone).not.toBeNull()
        expect(update).not.toHaveProperty('base_currency')
        expect(Object.keys(update).sort()).toEqual(
          companyFields.filter((field) => dirty[field]).sort(),
        )
      }
    }
  })
})

describe('companySchema', () => {
  const parse = (override: Partial<CompanyFormValues>) =>
    companySchema.safeParse({ ...values, ...override })
  const codes = (override: Partial<CompanyFormValues>) => {
    const result = parse(override)
    return result.success
      ? []
      : result.error.issues.map((issue) => issue.message)
  }
  it('accepts the saved values', () => {
    expect(parse({}).success).toBe(true)
  })
  it('trims and keeps empty optionals', () => {
    const result = parse({ tax_id: ' 30123456781 ', phone: '  ' })
    expect(result.success && result.data).toMatchObject({
      tax_id: '30123456781',
      phone: '',
    })
  })
  it('asks for a name and limits lengths with the contract maximums', () => {
    expect(codes({ name: '  ' })).toEqual(['required'])
    expect(codes({ name: 'x'.repeat(121) })).toEqual(['too_long'])
    expect(codes({ legal_name: 'x'.repeat(201) })).toEqual(['too_long'])
    expect(codes({ address: 'x'.repeat(301) })).toEqual(['too_long'])
    expect(codes({ phone: 'x'.repeat(51) })).toEqual(['too_long'])
    expect(parse({ legal_name: 'x'.repeat(200) }).success).toBe(true)
  })
  it('tells a malformed CUIT from one with a wrong check digit', () => {
    expect(codes({ tax_id: '3012345678' })).toEqual(['invalid_format'])
    expect(codes({ tax_id: '30123456782' })).toEqual(['invalid_tax_id'])
  })
  it('validates the contact email only when present', () => {
    expect(codes({ email: 'no-es-un-email' })).toEqual(['invalid_format'])
    expect(parse({ email: 'contacto@acme.test' }).success).toBe(true)
    expect(parse({ email: '' }).success).toBe(true)
  })
})
