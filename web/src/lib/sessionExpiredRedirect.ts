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
