import { expect, it } from 'vitest'
import { safeNextPath } from './safeNextPath'
import {
  sessionExpiredLoginPath,
  isSessionExpiredRedirect,
} from './sessionExpiredRedirect'
// T-F103 (INV-F07, INV-F18, DD-F32): only internal paths and canonical session redirects.
it.each([
  null,
  '',
  '//evil.example',
  '/\\evil.example',
  'https://evil.example',
  'javascript:alert(1)',
  'settings',
  '/%2F%2Fevil.example',
  '/%5cevil.example',
  '/%252f%252fevil.example',
])('rejects unsafe next %s', (raw) => expect(safeNextPath(raw)).toBe('/'))
it.each(['/settings/users', '/settings/users?x=1'])(
  'keeps internal next %s',
  (raw) => expect(safeNextPath(raw)).toBe(raw),
)
it.each([
  { pathname: '/settings/company', search: '' },
  { pathname: '/settings/users', search: '?x=1' },
])('creates and recognizes the expiration redirect from $pathname', (from) => {
  const path = sessionExpiredLoginPath(from)
  expect(path).toBe(
    `/login?next=${encodeURIComponent(from.pathname + from.search)}&reason=session_expired`,
  )
  const target = new URL(path, 'https://app.example')
  expect(safeNextPath(target.searchParams.get('next'))).toBe(
    from.pathname + from.search,
  )
  expect(isSessionExpiredRedirect(target)).toBe(true)
})
it.each([
  '/login',
  '/login?reason=logged_out',
  '/settings',
  '/settings?reason=session_expired',
])('does not confuse %s with expiration', (path) =>
  expect(isSessionExpiredRedirect(new URL(path, 'https://app.example'))).toBe(
    false,
  ),
)
