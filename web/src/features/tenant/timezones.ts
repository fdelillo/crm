const ARGENTINA = 'America/Argentina/'

export interface TimeZoneGroups {
  argentina: string[]
  others: string[]
}

/**
 * Previous names of the Argentine zones that IANA keeps as links (file `backward`): some
 * browsers list them instead of `America/Argentina/*`. A closed membership list, not an alias
 * table: it never translates a name. A new Argentine zone outside the prefix has to be added
 * here (ui.md §29).
 */
export const ARGENTINA_LEGACY_ZONES: ReadonlySet<string> = new Set([
  'America/Buenos_Aires',
  'America/Catamarca',
  'America/Cordoba',
  'America/Jujuy',
  'America/Mendoza',
  'America/Rosario',
])

const isArgentina = (zone: string) =>
  zone.startsWith(ARGENTINA) || ARGENTINA_LEGACY_ZONES.has(zone)

// DD-F27 (revised in P-F21): "Argentina" first, each group in the order of Intl. The saved zone
// is always offered, even when Intl does not list it: it goes in front of its own group.
export function timeZoneGroups(
  saved: string,
  supported: readonly string[] = Intl.supportedValuesOf('timeZone'),
): TimeZoneGroups {
  const zones =
    supported.includes(saved) || !saved ? supported : [saved, ...supported]
  return {
    argentina: zones.filter(isArgentina),
    others: zones.filter((zone) => !isArgentina(zone)),
  }
}
