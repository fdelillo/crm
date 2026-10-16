import { useEffect } from 'react'
import { useBlocker } from 'react-router'
import { isSessionExpiredRedirect } from './sessionExpiredRedirect'
// DD-F32: client editing state only; no server data is retained here.
let activeGuards = 0
export function hasUnsavedChanges(): boolean {
  return activeGuards > 0
}
export function useUnsavedChangesGuard(when: boolean) {
  useEffect(() => {
    if (!when) return
    activeGuards++
    return () => {
      activeGuards--
    }
  }, [when])
  return useBlocker(
    ({ nextLocation }) => when && !isSessionExpiredRedirect(nextLocation),
  )
}
