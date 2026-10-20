import { screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useLocation } from 'react-router'
import { useLinkToken } from './useLinkToken'
import { renderApp } from '@/test/renderApp'

function LinkTokenProbe() {
  const token = useLinkToken()
  const location = useLocation()
  return (
    <output aria-label="Token del enlace">
      {`${token ?? 'sin token'}|${location.hash}|${location.search}|${String(location.state?.linkToken ?? '')}`}
    </output>
  )
}

describe('useLinkToken', () => {
  // T-F401 (DD-14, DD-F2): removes link credentials from the visible URL and preserves same-tab reloads.
  it('moves a fragment token into router state and restores it after remount', async () => {
    const methods = ['debug', 'error', 'info', 'log', 'warn'] as const
    const consoleSpies = methods.map((method) =>
      vi.spyOn(console, method).mockImplementation(() => undefined),
    )
    const routes = [{ path: '/reset-password', Component: LinkTokenProbe }]
    const { router, unmount } = renderApp(['/reset-password#token=abc-_123'], {
      routes: [{ path: '/reset-password', Component: LinkTokenProbe }],
      strictMode: true,
    })
    expect(await screen.findByLabelText('Token del enlace')).toHaveTextContent(
      'abc-_123|||abc-_123',
    )
    await waitFor(() => expect(router.state.location.hash).toBe(''))
    expect(router.state.location.search).toBe('')
    expect(router.state.location.state).toEqual({ linkToken: 'abc-_123' })
    const savedState = router.state.location.state
    unmount()
    const remounted = renderApp(
      [{ pathname: '/reset-password', state: savedState }],
      { routes, strictMode: true },
    )
    expect(await screen.findByLabelText('Token del enlace')).toHaveTextContent(
      'abc-_123|||abc-_123',
    )
    expect(remounted.router.state.location.search).toBe('')
    for (const spy of consoleSpies) {
      expect(spy).not.toHaveBeenCalled()
      spy.mockRestore()
    }
  })

  it.each([
    '/reset-password',
    '/reset-password#token=',
    '/reset-password#foo=bar',
  ])('returns no token for %s', async (entry) => {
    renderApp([entry], {
      routes: [{ path: '/reset-password', Component: LinkTokenProbe }],
    })
    expect(await screen.findByLabelText('Token del enlace')).toHaveTextContent(
      'sin token',
    )
  })
})
