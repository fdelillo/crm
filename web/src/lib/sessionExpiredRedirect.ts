import type { Location } from 'react-router'
export interface SessionExpiredLoginState {
  unsavedChangesLost?: true
}
export function sessionExpiredLoginPath(
  from: Pick<Location, 'pathname' | 'search'>,
): string {
  return `/login?next=${encodeURIComponent(from.pathname + from.search)}&reason=session_expired`
}
export function isSessionExpiredRedirect(
  to: Pick<Location, 'pathname' | 'search'>,
): boolean {
  return (
    to.pathname === '/login' &&
    new URLSearchParams(to.search).get('reason') === 'session_expired'
  )
}
export type SessionExitReason =
  'session_expired' | 'logged_out' | 'password_changed'
export function sessionExitPath(
  reason: SessionExitReason,
  from: Pick<Location, 'pathname' | 'search'>,
): string {
  return reason === 'session_expired'
    ? sessionExpiredLoginPath(from)
    : `/login?reason=${reason}`
}
const exitReasons: readonly string[] = [
  'session_expired',
  'logged_out',
  'password_changed',
] satisfies SessionExitReason[]
export function isSessionExitRedirect(
  to: Pick<Location, 'pathname' | 'search'>,
): boolean {
  const reason = new URLSearchParams(to.search).get('reason')
  return (
    to.pathname === '/login' && reason !== null && exitReasons.includes(reason)
  )
}
