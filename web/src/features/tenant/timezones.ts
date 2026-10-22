const ARGENTINA = 'America/Argentina/'

export interface TimeZoneGroups {
  argentina: string[]
  others: string[]
}

/**
 * The seven Argentine links of the IANA file `backward`: previous name -> current zone. Some
 * browsers list the previous names instead of `America/Argentina/*`. The keys decide who belongs
 * to "Argentina" and the values decide who is the same zone under another name (DD-F27, P-F22).
 * A new Argentine zone outside the prefix, or a new link, has to be added here (ui.md §29).
 */
export const ARGENTINA_LEGACY_ZONES: ReadonlyMap<string, string> = new Map([
  ['America/Buenos_Aires', 'America/Argentina/Buenos_Aires'],
  ['America/Catamarca', 'America/Argentina/Catamarca'],
  ['America/Argentina/ComodRivadavia', 'America/Argentina/Catamarca'],
  ['America/Cordoba', 'America/Argentina/Cordoba'],
  ['America/Rosario', 'America/Argentina/Cordoba'],
  ['America/Jujuy', 'America/Argentina/Jujuy'],
  ['America/Mendoza', 'America/Argentina/Mendoza'],
])

const isArgentina = (zone: string) =>
  zone.startsWith(ARGENTINA) || ARGENTINA_LEGACY_ZONES.has(zone)

const currentZoneName = (zone: string) =>
  ARGENTINA_LEGACY_ZONES.get(zone) ?? zone

// DD-F27 (revised in P-F21 and P-F22): "Argentina" first, each group in the order of Intl. The
// saved zone is always offered, even when Intl does not list it: it goes in front of its own
// group, and the listed zone that is the same one under another name is left out. The saved name
// is the value of the form, so showing it keeps the field untouched (INV-F15).
export function timeZoneGroups(
  saved: string,
  supported: readonly string[] = Intl.supportedValuesOf('timeZone'),
): TimeZoneGroups {
  const zones =
    supported.includes(saved) || !saved
      ? supported
      : [
          saved,
          ...supported.filter(
            (zone) => currentZoneName(zone) !== currentZoneName(saved),
          ),
        ]
  return {
    argentina: zones.filter(isArgentina),
    others: zones.filter((zone) => !isArgentina(zone)),
  }
}
