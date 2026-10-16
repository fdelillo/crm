import { useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Outlet, ScrollRestoration, useLocation } from 'react-router'
import { OfflineBanner } from '@/components/OfflineBanner'
import { queryKeys } from '@/api/queryKeys'
import type { SessionInfo } from '@/api/types'
export function RootLayout() {
  const location = useLocation()
  const previousPath = useRef(location.pathname)
  const client = useQueryClient()
  useEffect(() => {
    const changed = previousPath.current !== location.pathname
    previousPath.current = location.pathname
    const update = () => {
      const heading = document.querySelector('h1')
      if (!heading) return false
      const session = client.getQueryData<SessionInfo | null>(queryKeys.session)
      document.title =
        heading.textContent + (session ? ` · ${session.tenant.name}` : '')
      if (changed) {
        heading.tabIndex = -1
        heading.focus()
      }
      return true
    }
    if (update()) return
    const observer = new MutationObserver(() => {
      if (update()) observer.disconnect()
    })
    observer.observe(document.body, { childList: true, subtree: true })
    return () => observer.disconnect()
  }, [location.pathname, client])
  return (
    <>
      <OfflineBanner />
      <Outlet />
      <ScrollRestoration />
    </>
  )
}
