import { afterEach, describe, expect, it, vi } from 'vitest'
import { formatDateTime, formatRetryAt } from './format'

// Local-time constructors keep these cases independent of the machine's time zone.
const at = (h: number, m: number, s = 0) => new Date(2026, 9, 16, h, m, s)
const clock = (d: Date) => d.toTimeString().slice(0, 8)

afterEach(() => vi.useRealTimers())

// T-F101 (P-F16, P-F17), ui.md §13.2: next-minute rounding and wall-clock minutes, minimum 1.
describe('formatRetryAt', () => {
  it.each([
    [900, at(14, 20, 0), { time: '14:35', minutes: 15 }],
    [900, at(14, 20, 30), { time: '14:36', minutes: 16 }],
    [10, at(14, 20, 30), { time: '14:21', minutes: 1 }],
    [60, at(14, 20, 0), { time: '14:21', minutes: 1 }],
    [900, at(23, 50, 0), { time: '00:05', minutes: 15 }],
  ])('retry in %i s', (seconds, now, expected) => {
    expect(formatRetryAt(seconds, now), clock(now)).toEqual(expected)
  })
  it('does not mutate the received date', () => {
    const now = at(14, 20, 30)
    formatRetryAt(900, now)
    expect(now.getTime()).toBe(at(14, 20, 30).getTime())
  })
})

// T-F501 (ADR-023, DD-25): dates use the company zone and include the year only when needed.
describe('formatDateTime', () => {
  it('formats invitation expiry in the company time zone and omits the current year', () => {
    expect(
      formatDateTime(
        '2026-10-06T17:30:00Z',
        'America/Argentina/Buenos_Aires',
        new Date('2026-09-29T12:00:00Z'),
      ),
    ).toBe('6/10, 14:30')
  })
  it('includes the year when the invitation expires in another year', () => {
    expect(
      formatDateTime(
        '2026-10-06T17:30:00Z',
        'America/Argentina/Buenos_Aires',
        new Date('2025-09-29T12:00:00Z'),
      ),
    ).toBe('6/10/2026, 14:30')
  })
})
