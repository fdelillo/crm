import { useEffect, useLayoutEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Outlet, ScrollRestoration, useLocation } from 'react-router'
import { OfflineBanner } from '@/components/OfflineBanner'
import { queryKeys } from '@/api/queryKeys'
import { consumeErrorScreenExit } from './errorScreenExit'
import type { SessionInfo } from '@/api/types'
export function RootLayout() {
  const location = useLocation()
  const previousPath = useRef(location.pathname)
  const client = useQueryClient()
  const pendingFocus = useRef(false)
  const lastHeading = useRef<HTMLHeadingElement | null>(null)
  // Leaving the error screen (any way) remounts this layout; the boundary left a
  // flag. Read in an effect, not in render: StrictMode runs the second mount
  // effect with the flag already consumed, and it must not overwrite the true.
  useLayoutEffect(() => {
    if (consumeErrorScreenExit()) pendingFocus.current = true
  }, [])
  // Layout effect: mark the navigation before the observer sees the new <h1>.
  useLayoutEffect(() => {
    if (previousPath.current !== location.pathname) pendingFocus.current = true
    previousPath.current = location.pathname
  }, [location.pathname])
  useEffect(() => {
    const update = () => {
      const heading = document.querySelector('h1')
      if (!heading) return
      const session = client.getQueryData<SessionInfo | null>(queryKeys.session)
      const title = `${heading.textContent} · ${session ? session.tenant.name : 'CRM'}`
      if (document.title !== title) document.title = title
      if (heading === lastHeading.current) {
        // React may reuse the same <h1> node for another route's screen.
        if (pendingFocus.current) {
          pendingFocus.current = false
          heading.tabIndex = -1
          heading.focus()
        }
        return
      }
      // A new screen is presented: after navigation, or in place of one whose
      // recovery control (e.g. "Reintentar") just took the focus away with it.
      const previous = lastHeading.current
      const focusLost =
        previous !== null &&
        !previous.isConnected &&
        (document.activeElement === null ||
          document.activeElement === document.body)
      lastHeading.current = heading
      if (pendingFocus.current || focusLost) {
        pendingFocus.current = false
        heading.tabIndex = -1
        heading.focus()
      }
    }
    update()
    const observer = new MutationObserver(update)
    observer.observe(document.body, {
      childList: true,
      subtree: true,
      characterData: true,
    })
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
