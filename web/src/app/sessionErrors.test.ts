import { QueryClient } from '@tanstack/react-query'
import type { Location } from 'react-router'
import { describe, expect, it, vi } from 'vitest'
import { createSessionErrorHandlers } from './sessionErrors'

const currentLocation: Location = {
  pathname: '/settings',
  search: '',
  hash: '',
  state: null,
  key: 'current',
}

function setup(navigationLocation?: Location) {
  const client = new QueryClient()
  const clear = vi.spyOn(client, 'clear')
  const navigate = vi.fn(async () => {})
  const router = {
    state: {
      location: currentLocation,
      navigation: {
        ...(navigationLocation ? { location: navigationLocation } : {}),
      },
    },
    navigate,
  }
  return {
    client,
    clear,
    navigate,
    handlers: createSessionErrorHandlers(client, router),
  }
}

describe('createSessionErrorHandlers', () => {
  it('does not expire the session while navigation to login is pending', () => {
    const pendingLogin: Location = {
      ...currentLocation,
      pathname: '/login',
      key: 'pending-login',
    }
    const { clear, navigate, handlers } = setup(pendingLogin)

    handlers.onUnauthenticated()

    expect(clear).not.toHaveBeenCalled()
    expect(navigate).not.toHaveBeenCalled()
  })

  it('expires the session when navigation is idle', () => {
    sessionStorage.setItem('crm.verifyBanner.dismissed', 'true')
    const { clear, navigate, handlers } = setup()

    handlers.onUnauthenticated()

    expect(clear).toHaveBeenCalledOnce()
    expect(sessionStorage.getItem('crm.verifyBanner.dismissed')).toBeNull()
    expect(navigate).toHaveBeenCalledOnce()
  })
})
