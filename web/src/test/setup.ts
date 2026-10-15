import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { beforeAll, afterEach, afterAll } from 'vitest'
import { server } from './msw/server'
import { clearTestQueryClients } from './queryClient'

// MSW 3 renames the unhandled-request policy to onUnhandledFrame.
beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  cleanup()
  clearTestQueryClients()
  server.resetHandlers()
  sessionStorage.clear()
})
afterAll(() => server.close())
