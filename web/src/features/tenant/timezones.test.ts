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

// T-F602 (DD-F27, S-F11, P-F21, P-F22)
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
  it('knows exactly the seven links to Argentine zones that IANA keeps, each with its current zone', () => {
    expect(Object.fromEntries(ARGENTINA_LEGACY_ZONES)).toEqual({
      'America/Buenos_Aires': 'America/Argentina/Buenos_Aires',
      'America/Catamarca': 'America/Argentina/Catamarca',
      'America/Argentina/ComodRivadavia': 'America/Argentina/Catamarca',
      'America/Cordoba': 'America/Argentina/Cordoba',
      'America/Rosario': 'America/Argentina/Cordoba',
      'America/Jujuy': 'America/Argentina/Jujuy',
      'America/Mendoza': 'America/Argentina/Mendoza',
    })
  })
  it('puts the default of the backend first in "Argentina" without repeating it under its previous name', () => {
    expect(
      timeZoneGroups('America/Argentina/Buenos_Aires', supported).argentina,
    ).toEqual([
      'America/Argentina/Buenos_Aires',
      'America/Argentina/Cordoba',
      'America/Mendoza',
    ])
  })
  it('puts a saved previous name first and hides the listed zone that is the same one', () => {
    const groups = timeZoneGroups('America/Rosario', supported)
    expect(groups.argentina).toEqual([
      'America/Rosario',
      'America/Buenos_Aires',
      'America/Mendoza',
    ])
    expect(groups.argentina).not.toContain('America/Argentina/Cordoba')
    expect(groups.others).not.toContain('America/Rosario')
  })
  it('hides the previous name when the browser lists the current names', () => {
    const current = [
      'America/Argentina/Buenos_Aires',
      'America/Argentina/Cordoba',
      'Europe/Madrid',
    ]
    expect(timeZoneGroups('America/Buenos_Aires', current)).toEqual({
      argentina: ['America/Buenos_Aires', 'America/Argentina/Cordoba'],
      others: ['Europe/Madrid'],
    })
  })
  it('does not deduplicate outside Argentina: both names stay, the saved one first', () => {
    expect(
      timeZoneGroups('Asia/Kolkata', ['Asia/Calcutta', 'Europe/Madrid']),
    ).toEqual({
      argentina: [],
      others: ['Asia/Kolkata', 'Asia/Calcutta', 'Europe/Madrid'],
    })
  })
  it('hides nothing when the saved zone is listed', () => {
    const listed = [
      'America/Argentina/Buenos_Aires',
      'America/Buenos_Aires',
      'America/Argentina/Cordoba',
    ]
    expect(timeZoneGroups('America/Buenos_Aires', listed).argentina).toEqual(
      listed,
    )
    expect(
      timeZoneGroups('America/Argentina/Buenos_Aires', listed).argentina,
    ).toEqual(listed)
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
