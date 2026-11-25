import { describe, expect, it } from 'vitest'

import {
  MAX_AMOUNT_CENTS,
  formatMoney,
  formatMoneyInput,
  parseMoneyInput,
} from './money'

// T-002-F102 (002/INV-F5, DD-F8): the table of ui.md §17.2, row by row.
describe('parseMoneyInput', () => {
  it.each([
    ['4500', 450000],
    ['4.500', 450000],
    ['4500,5', 450050],
    ['4.500,50', 450050],
    ['0', 0],
    ['0,00', 0],
    ['4500.50', 450050],
    ['1.5', 150],
    ['  4.500,50  ', 450050],
    ['90.071.992.547.409,91', 9007199254740991],
    // A float conversion (Math.round(parseFloat(x) * 100)) loses these two.
    ['90.071.992.547.409,90', 9007199254740990],
    ['90.071.992.547.409,87', 9007199254740987],
  ])('accepts %j as %i cents', (text, amountCents) => {
    expect(parseMoneyInput(text)).toEqual({ ok: true, amountCents })
  })

  it.each(['', '   '])('reports %j as empty', (text) => {
    expect(parseMoneyInput(text)).toEqual({ ok: false, reason: 'empty' })
  })

  it.each(['1.50.000', '4,500.50', '4.50,00', '1,234', '-5', '$ 100', '1e3'])(
    'rejects %j as a format error',
    (text) => {
      expect(parseMoneyInput(text)).toEqual({ ok: false, reason: 'format' })
    },
  )

  it.each([
    '90.071.992.547.409,92',
    '90.071.992.547.410',
    '99999999999999999999',
  ])('rejects %j as too large', (text) => {
    expect(parseMoneyInput(text)).toEqual({ ok: false, reason: 'too_large' })
  })

  it('exposes the exact maximum', () => {
    expect(MAX_AMOUNT_CENTS).toBe(9007199254740991)
  })
})

describe('formatMoney', () => {
  it('matches Intl as oracle for the currency code (S-F2)', () => {
    const expected = new Intl.NumberFormat('es-AR', {
      style: 'currency',
      currency: 'USD',
      currencyDisplay: 'code',
    }).format('4500.00' as unknown as number)
    const actual = formatMoney(450000, 'USD')
    expect(actual).toBe(expected)
    expect(actual).toContain('USD')
    expect(actual).toContain('4.500,00')
  })

  it('keeps cents below one unit', () => {
    expect(formatMoney(5, 'ARS')).toContain('0,05')
    expect(formatMoney(0, 'ARS')).toContain('0,00')
  })

  // Near 2^53 a double cannot hold every cent: formatting through a number shows the wrong
  // cent for some of these amounts (.91 and .87 on Node), so the three rows together catch it.
  it.each([
    [9007199254740991, '90.071.992.547.409,91'],
    [9007199254740990, '90.071.992.547.409,90'],
    [9007199254740987, '90.071.992.547.409,87'],
  ])('is exact near 2^53 - 1: %i', (cents, text) => {
    expect(formatMoney(cents, 'ARS')).toContain(text)
  })

  it.each([-1, 1.5, 9007199254740992, Number.NaN])(
    'throws for %s (programming error)',
    (cents) => {
      expect(() => formatMoney(cents, 'ARS')).toThrow()
    },
  )
})

describe('formatMoneyInput', () => {
  it('formats cents for prefilling a field', () => {
    expect(formatMoneyInput(450050)).toBe('4.500,50')
  })

  it.each([
    0, 5, 99, 100, 123456, 9007199254740987, 9007199254740990, 9007199254740991,
  ])('round-trips %i through parseMoneyInput', (cents) => {
    expect(parseMoneyInput(formatMoneyInput(cents))).toEqual({
      ok: true,
      amountCents: cents,
    })
  })
})
