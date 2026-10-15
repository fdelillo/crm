import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { beforeAll, afterEach, afterAll, vi } from 'vitest'
import { server } from './msw/server'
import { clearTestQueryClients } from './queryClient'
import {
  assertNoUnhandledRequests,
  recordUnhandledRequest,
} from './unhandledRequests'

// MSW 3 renames the unhandled-request policy to onUnhandledFrame (ADR-022: 'error').
// The event records the request so afterEach can fail the test.
beforeAll(() => {
  server.events.on('request:unhandled', ({ request }) =>
    recordUnhandledRequest(request),
  )
  server.listen({ onUnhandledFrame: 'error' })
})
Object.defineProperty(window, 'scrollTo', {
  configurable: true,
  value: vi.fn(),
})
afterEach(() => {
  // First, so a failure while cleaning up cannot leave the pending list for the next test.
  assertNoUnhandledRequests()
  cleanup()
  clearTestQueryClients()
  server.resetHandlers()
  sessionStorage.clear()
})
afterAll(() => server.close())
