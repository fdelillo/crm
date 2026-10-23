import { expect, it } from 'vitest'
import { safeNextPath } from './safeNextPath'
import {
  isSessionExitRedirect,
  sessionExitPath,
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

// T-F707 (DD-F35, INV-F18): one place builds and recognizes every session-exit URL.
const settings = { pathname: '/settings', search: '?x=1' }
it('builds the expiration exit like sessionExpiredLoginPath', () =>
  expect(sessionExitPath('session_expired', settings)).toBe(
    sessionExpiredLoginPath(settings),
  ))
it.each(['logged_out', 'password_changed'] as const)(
  'builds the %s exit without next',
  (reason) =>
    expect(sessionExitPath(reason, settings)).toBe(`/login?reason=${reason}`),
)
it.each([
  sessionExpiredLoginPath(settings),
  '/login?reason=logged_out',
  '/login?reason=password_changed',
])('recognizes %s as a session exit', (path) =>
  expect(isSessionExitRedirect(new URL(path, 'https://app.example'))).toBe(
    true,
  ),
)
it.each(['/login?reason=other', '/login', '/settings?reason=logged_out'])(
  'does not recognize %s as a session exit',
  (path) =>
    expect(isSessionExitRedirect(new URL(path, 'https://app.example'))).toBe(
      false,
    ),
)
