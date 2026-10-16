import { afterEach, describe, expect, it, vi } from 'vitest'
import { formatRetryAt } from './format'

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
