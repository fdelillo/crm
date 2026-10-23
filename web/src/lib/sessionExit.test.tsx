import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import {
  attachSessionExitRouter,
  exitSession,
  markSessionPresented,
  sessionLifecycleOf,
  useSessionLifecycle,
} from './sessionExit'

// T-F707 (DD-F35, INV-F22): a finished exit is cleared when the last screen that follows the
// lifecycle unmounts, not when the router publishes /login; the pure registry, no UI.
function setup() {
  const client = new QueryClient()
  const state = { location: { pathname: '/settings', search: '' } }
  const router = {
    state,
    navigate: vi.fn(async (to: string) => {
      const url = new URL(to, 'https://app.example')
      state.location = { pathname: url.pathname, search: url.search }
    }),
  }
  attachSessionExitRouter(client, router)
  markSessionPresented(client)
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  return { client, wrapper }
}
const settle = () => act(async () => {})

describe('session exit registry', () => {
  it('clears at once when no screen follows the lifecycle', async () => {
    const { client } = setup()
    exitSession(client, 'logged_out')
    await settle()
    expect(sessionLifecycleOf(client)).toEqual({
      presented: false,
      exit: null,
    })
  })

  it('keeps the exit after reaching /login until the last follower unmounts', async () => {
    const { client, wrapper } = setup()
    const first = renderHook(() => useSessionLifecycle(), { wrapper })
    const second = renderHook(() => useSessionLifecycle(), { wrapper })
    exitSession(client, 'session_expired')
    await settle()
    expect(first.result.current.exit?.status).toBe('navigating')
    first.unmount()
    expect(sessionLifecycleOf(client).exit).not.toBeNull()
    second.unmount()
    expect(sessionLifecycleOf(client)).toEqual({
      presented: false,
      exit: null,
    })
  })

  it('does not subscribe again on re-render, which would clear the exit early', async () => {
    const { client, wrapper } = setup()
    const view = renderHook(() => useSessionLifecycle(), { wrapper })
    exitSession(client, 'logged_out')
    await settle()
    view.rerender()
    view.rerender()
    expect(sessionLifecycleOf(client).exit).not.toBeNull()
    expect(view.result.current.exit?.to).toBe('/login?reason=logged_out')
  })
})
