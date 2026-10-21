import { useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router'

/** Reads an email-link fragment once, then keeps it only in the current history entry. */
export function useLinkToken(): string | null {
  const location = useLocation()
  const navigate = useNavigate()
  const stateToken =
    typeof location.state?.linkToken === 'string'
      ? location.state.linkToken
      : null
  const match = /^#token=([A-Za-z0-9_-]+)$/.exec(location.hash)
  const fragmentToken = match?.[1] ?? null

  useEffect(() => {
    if (!fragmentToken) return
    void navigate(location.pathname, {
      replace: true,
      state: { ...location.state, linkToken: fragmentToken },
    })
  }, [fragmentToken, location.pathname, location.state, navigate])

  return stateToken ?? fragmentToken
}
