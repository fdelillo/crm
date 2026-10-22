import { describe, expect, it } from 'vitest'
import { ARGENTINA_LEGACY_ZONES, timeZoneGroups } from './timezones'

const supported = [
  'Africa/Abidjan',
  'America/Argentina/Cordoba',
  'America/Buenos_Aires',
  'America/Mendoza',
  'America/Santiago',
  'Europe/Madrid',
]

// T-F602 (DD-F27, S-F11, P-F21)
describe('timeZoneGroups', () => {
  it('groups the previous names of the Argentine zones with America/Argentina/*, in the order of Intl', () => {
    expect(timeZoneGroups('Europe/Madrid', supported)).toEqual({
      argentina: [
        'America/Argentina/Cordoba',
        'America/Buenos_Aires',
        'America/Mendoza',
      ],
      others: ['Africa/Abidjan', 'America/Santiago', 'Europe/Madrid'],
    })
  })
  it('knows exactly the six previous names IANA keeps as links', () => {
    expect([...ARGENTINA_LEGACY_ZONES].sort()).toEqual([
      'America/Buenos_Aires',
      'America/Catamarca',
      'America/Cordoba',
      'America/Jujuy',
      'America/Mendoza',
      'America/Rosario',
    ])
  })
  it('puts a saved zone that Intl does not list first in "Argentina" when it is the default of the backend', () => {
    const groups = timeZoneGroups('America/Argentina/Buenos_Aires', supported)
    expect(groups.argentina[0]).toBe('America/Argentina/Buenos_Aires')
    expect(groups.argentina).toHaveLength(4)
  })
  it('puts a saved previous name that Intl does not list first in "Argentina"', () => {
    const groups = timeZoneGroups('America/Rosario', supported)
    expect(groups.argentina[0]).toBe('America/Rosario')
    expect(groups.others).not.toContain('America/Rosario')
  })
  it('puts any other saved zone that Intl does not list first in "Otras zonas"', () => {
    const groups = timeZoneGroups('Europe/Berlin', supported)
    expect(groups.others[0]).toBe('Europe/Berlin')
    expect(groups.argentina).not.toContain('Europe/Berlin')
  })
  it('does not repeat a zone that is already listed', () => {
    const groups = timeZoneGroups('America/Argentina/Cordoba', supported)
    expect([...groups.argentina, ...groups.others]).toHaveLength(6)
  })
  it('reads the browser list by default', () => {
    const groups = timeZoneGroups('America/Argentina/Buenos_Aires')
    expect(groups.argentina.length).toBeGreaterThan(0)
  })
})
