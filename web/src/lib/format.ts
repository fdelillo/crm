const MINUTE_MS = 60_000

// Hour of the retry in the browser's time zone (no session, so no company zone: ADR-023, INV-F12).
const clockFormat = new Intl.DateTimeFormat('es-AR', {
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
})

// T-F102 (P-F16, P-F17), ui.md §13.2: rounds up to the next minute (an exact minute does not move);
// minutes are wall-clock minutes from `now` truncated to the minute, never less than 1.
export function formatRetryAt(
  retryAfterSeconds: number,
  now: Date,
): { time: string; minutes: number } {
  const target = now.getTime() + retryAfterSeconds * 1000
  const shown = Math.ceil(target / MINUTE_MS) * MINUTE_MS
  const nowMinute = Math.floor(now.getTime() / MINUTE_MS) * MINUTE_MS
  return {
    time: clockFormat.format(shown),
    minutes: Math.max(1, Math.round((shown - nowMinute) / MINUTE_MS)),
  }
}
