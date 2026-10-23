import { useSyncExternalStore } from 'react'
import type { QueryClient } from '@tanstack/react-query'
import { useQueryClient } from '@tanstack/react-query'
import type { Location, NavigateOptions } from 'react-router'
import {
  sessionExitPath,
  type SessionExitReason,
  type SessionExpiredLoginState,
} from './sessionExpiredRedirect'
import { hasUnsavedChanges } from './unsavedChanges'
import { clearVerifyBannerDismissed } from './verifyBannerDismissal'

// DD-F35, INV-F22 (ui.md §6.4.2): the only way out of a session. One registry per
// QueryClient says whether the tab presented a session and which exit is under way.
// It never holds SessionInfo (INV-F01): the session itself stays in the query cache.
export interface SessionExit {
  to: string
  state?: SessionExpiredLoginState
  status: 'navigating' | 'stalled'
}
export interface SessionLifecycle {
  presented: boolean
  exit: SessionExit | null
}
export interface SessionExitRouter {
  state: {
    location: Pick<Location, 'pathname' | 'search'>
    navigation?: { state?: string }
  }
  navigate: (to: string, options: NavigateOptions) => Promise<void>
  /** The real router has it; the double of sessionErrors.test.ts does not. */
  subscribe?: (
    listener: (state: {
      location: Pick<Location, 'pathname' | 'search'>
      navigation: { state?: string }
    }) => void,
  ) => () => void
}

interface Registry {
  lifecycle: SessionLifecycle
  router: SessionExitRouter | null
  listeners: Set<() => void>
  // The navigation reached /login while a screen that follows the lifecycle was still mounted.
  settleOnRelease: boolean
}
const registries = new WeakMap<QueryClient, Registry>()

function registryOf(client: QueryClient): Registry {
  let registry = registries.get(client)
  if (!registry) {
    registry = {
      lifecycle: { presented: false, exit: null },
      router: null,
      listeners: new Set(),
      settleOnRelease: false,
    }
    registries.set(client, registry)
  }
  return registry
}
function update(registry: Registry, next: Partial<SessionLifecycle>) {
  registry.lifecycle = { ...registry.lifecycle, ...next }
  for (const listener of [...registry.listeners]) listener()
}

export function attachSessionExitRouter(
  client: QueryClient,
  router: SessionExitRouter,
): void {
  registryOf(client).router = router
}

export function markSessionPresented(client: QueryClient): void {
  const registry = registryOf(client)
  if (!registry.lifecycle.presented) update(registry, { presented: true })
}

export function sessionLifecycleOf(client: QueryClient): SessionLifecycle {
  return registryOf(client).lifecycle
}

export function useSessionLifecycle(): SessionLifecycle {
  const registry = registryOf(useQueryClient())
  return useSyncExternalStore(
    (listener) => {
      registry.listeners.add(listener)
      return () => {
        registry.listeners.delete(listener)
        if (registry.settleOnRelease && registry.listeners.size === 0)
          arrive(registry)
      }
    },
    () => registry.lifecycle,
  )
}

function arrive(registry: Registry) {
  registry.settleOnRelease = false
  update(registry, { exit: null, presented: false })
}

function navigateOut(registry: Registry, exit: SessionExit) {
  const router = registry.router
  if (!router) throw new Error('exitSession needs a router: attach it first')
  const subscription: { stop?: () => void } = {}
  const finish = () => {
    subscription.stop?.()
    // Another exit (or a resume) may own the registry by now.
    if (registry.lifecycle.exit !== exit) return
    if (router.state.location.pathname === '/login') {
      // The router publishes the location before React unmounts the guarded screen.
      // Settling now would let that screen see "no session, none presented" and
      // navigate to /login?next= without the reason, so wait until it is gone.
      if (registry.listeners.size > 0) registry.settleOnRelease = true
      else arrive(registry)
    } else update(registry, { exit: { ...exit, status: 'stalled' } })
  }
  const navigation = router.navigate(exit.to, {
    replace: true,
    ...(exit.state ? { state: exit.state } : {}),
  })
  // S-F16 does not hold while a lazy route is loading: the promise of an interrupted navigation
  // settles only when that chunk arrives, so the router going idle is what ends the exit.
  subscription.stop = router.subscribe?.((state) => {
    if (state.navigation.state === 'idle') finish()
  })
  void Promise.resolve(navigation).then(finish, finish)
}

export function exitSession(
  client: QueryClient,
  reason: SessionExitReason,
): void {
  const registry = registryOf(client)
  if (registry.lifecycle.exit) return
  const router = registry.router
  if (!router) throw new Error('exitSession needs a router: attach it first')
  const unsavedChangesLost = reason === 'session_expired' && hasUnsavedChanges()
  const exit: SessionExit = {
    to: sessionExitPath(reason, router.state.location),
    ...(unsavedChangesLost ? { state: { unsavedChangesLost: true } } : {}),
    status: 'navigating',
  }
  update(registry, { exit })
  client.clear()
  clearVerifyBannerDismissed()
  navigateOut(registry, exit)
}

export function resumeSessionExit(client: QueryClient): void {
  const registry = registryOf(client)
  const stalled = registry.lifecycle.exit
  if (stalled?.status !== 'stalled') return
  const exit: SessionExit = { ...stalled, status: 'navigating' }
  update(registry, { exit })
  navigateOut(registry, exit)
}
