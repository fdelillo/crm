const ARGENTINA = 'America/Argentina/'

export interface TimeZoneGroups {
  argentina: string[]
  others: string[]
}

// DD-F27: Argentina first; the saved zone is always offered, even when Intl does not list it
// (S-F11: some browsers return an alias). There is no alias table: a saved zone Intl does not
// list goes in front of its group.
export function timeZoneGroups(
  saved: string,
  supported: readonly string[] = Intl.supportedValuesOf('timeZone'),
): TimeZoneGroups {
  const zones =
    supported.includes(saved) || !saved ? supported : [saved, ...supported]
  return {
    argentina: zones.filter((zone) => zone.startsWith(ARGENTINA)),
    others: zones.filter((zone) => !zone.startsWith(ARGENTINA)),
  }
}
