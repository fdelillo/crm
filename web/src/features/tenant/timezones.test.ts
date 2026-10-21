import { describe, expect, it } from 'vitest'
import { timeZoneGroups } from './timezones'

const supported = [
  'Africa/Abidjan',
  'America/Argentina/Buenos_Aires',
  'America/Argentina/Cordoba',
  'Europe/Madrid',
]

// T-F602 (DD-F27, S-F11)
describe('timeZoneGroups', () => {
  it('puts America/Argentina/* in its own group and keeps the rest in order', () => {
    expect(timeZoneGroups('Europe/Madrid', supported)).toEqual({
      argentina: [
        'America/Argentina/Buenos_Aires',
        'America/Argentina/Cordoba',
      ],
      others: ['Africa/Abidjan', 'Europe/Madrid'],
    })
  })
  it('always offers the saved zone, even if the browser does not list it', () => {
    const groups = timeZoneGroups('America/Buenos_Aires', supported)
    expect([...groups.argentina, ...groups.others]).toContain(
      'America/Buenos_Aires',
    )
    expect(groups.others[0]).toBe('America/Buenos_Aires')
  })
  it('does not repeat a zone that is already listed', () => {
    const groups = timeZoneGroups('America/Argentina/Cordoba', supported)
    expect([...groups.argentina, ...groups.others]).toHaveLength(4)
  })
  it('reads the browser list by default', () => {
    const groups = timeZoneGroups('America/Argentina/Buenos_Aires')
    expect(groups.argentina.length).toBeGreaterThan(0)
  })
})
