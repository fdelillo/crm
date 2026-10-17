import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { beforeAll, afterEach, afterAll, vi } from 'vitest'
import { resetErrorScreenExit } from '@/app/errorScreenExit'
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

// Radix RadioGroup measures its hidden form input; jsdom has no layout observer.
class ResizeObserverStub implements ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
Object.defineProperty(globalThis, 'ResizeObserver', {
  configurable: true,
  value: ResizeObserverStub,
})
afterEach(() => {
  // Checked first so a failure while cleaning up cannot leave the pending list
  // for the next test; the finally block cleans up even when the check throws.
  try {
    assertNoUnhandledRequests()
  } finally {
    cleanup()
    clearTestQueryClients()
    server.resetHandlers()
    sessionStorage.clear()
    resetErrorScreenExit()
  }
})
afterAll(() => server.close())
