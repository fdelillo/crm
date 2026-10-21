import { describe, expect, it } from 'vitest'
import { isValidCuit } from './cuit'

// T-F601 (DD-16, ui.md §13.11): same cases as the backend table in
// internal/tenant/validation_test.go (T-B701). Source: ARCA's official validator,
// https://seti.afip.gob.ar/padron-puc-constancia-internet/js/ValidaCuit.js
// (weighted sum including the check digit must be divisible by 11). Fixtures are synthetic.
describe('isValidCuit', () => {
  it.each([
    ['30-12345678-1', true, 'valid with hyphens'],
    ['30123456781', true, 'valid without hyphens'],
    ['30-00000009-0', true, 'sum 33: 11 - remainder == 11 means digit 0'],
    ['30000000049', false, 'sum 23: 11 - remainder == 10 has no digit'],
    ['30000000040', false, 'remainder 10: digit 0 does not repair it'],
    ['30000000030', false, 'remainder 10: digit 0 does not repair it (bis)'],
    ['30123456782', false, 'wrong check digit'],
    ['3012345678', false, 'ten digits'],
    ['301234567812', false, 'twelve digits'],
    ['30abcdefgh1', false, 'letters'],
    ['30--123456781', false, 'double hyphen'],
    ['3-0123456789-1', false, 'hyphens in another position'],
    ['301-2345678-1', false, 'hyphens in another position (bis)'],
    ['30-123456781', false, 'a single hyphen'],
    ['', false, 'empty'],
    [' 30123456781', false, 'the server does not trim; the form trims before'],
  ])('%s -> %s (%s)', (raw, expected) => {
    expect(isValidCuit(raw)).toBe(expected)
  })
})
