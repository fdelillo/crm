import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { QueryClient } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { server } from './msw/server'
import { createTestQueryClient } from './queryClient'
import { assertNoUnhandledRequests } from './unhandledRequests'

const origin = window.location.origin
// Survives between tests on purpose: the next test checks that setup cleared it.
let cacheFromPreviousTest: QueryClient | undefined

// T-F003 (S-F2): real Request/fetch and Radix controls work in the browser test harness.
describe('browser test harness', () => {
  it('intercepts an absolute Request and returns handler JSON', async () => {
    server.use(
      http.get(`${origin}/api/v1/me`, () =>
        HttpResponse.json({ name: 'Persona ficticia' }),
      ),
    )
    const response = await fetch(new Request(`${origin}/api/v1/me`))
    expect(await response.json()).toEqual({ name: 'Persona ficticia' })
  })
  it('reports the method and URL for an unhandled request', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    await expect(
      fetch(
        new Request(`${origin}/api/v1/missing-handler`, { method: 'POST' }),
      ),
    ).rejects.toThrow()
    // The same check runs in afterEach, so a caught network error cannot hide it.
    expect(assertNoUnhandledRequests).toThrow(
      /POST .*\/api\/v1\/missing-handler/,
    )
  })
  // Fails only if the afterEach in setup.ts checks the unhandled list: here the
  // test's own code swallowed the network error and asserts nothing.
  it.fails(
    'afterEach fails a test whose code caught an unhandled request',
    async () => {
      vi.spyOn(console, 'error').mockImplementation(() => undefined)
      await fetch(`${origin}/api/v1/caught`).catch(() => undefined)
    },
  )
  // Pair: A fails in afterEach and must still be cleaned up; B checks the leftovers.
  it.fails(
    'afterEach fails an unhandled request and still cleans up',
    async () => {
      vi.spyOn(console, 'error').mockImplementation(() => undefined)
      render(<p>Residuo del test anterior</p>)
      sessionStorage.setItem('leftover', 'yes')
      server.use(
        http.get(`${origin}/api/v1/leftover`, () => HttpResponse.json({})),
      )
      await fetch(`${origin}/api/v1/unhandled-leftover`).catch(() => undefined)
    },
  )
  it('starts clean after a test that failed in afterEach', async () => {
    expect(screen.queryByText('Residuo del test anterior')).toBeNull()
    expect(sessionStorage.length).toBe(0)
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    await expect(fetch(`${origin}/api/v1/leftover`)).rejects.toThrow()
    expect(assertNoUnhandledRequests).toThrow(/api\/v1\/leftover/)
  })
  // The next two tests are a pair and rely on running in this order (Vitest runs
  // a file's tests sequentially unless told otherwise): the first leaves state, the second checks it is gone.
  it('can populate a test cache, override a handler and save a temporary flag', () => {
    cacheFromPreviousTest = createTestQueryClient()
    cacheFromPreviousTest.setQueryData(['session'], {
      name: 'Persona ficticia',
    })
    sessionStorage.setItem('test-flag', 'yes')
    server.use(
      http.get(`${origin}/api/v1/isolation`, () =>
        HttpResponse.json({ temporary: true }),
      ),
    )
  })
  it('starts the next test with clean cache, handlers and session storage', async () => {
    expect(cacheFromPreviousTest).toBeDefined()
    expect(cacheFromPreviousTest?.getQueryData(['session'])).toBeUndefined()
    expect(sessionStorage.length).toBe(0)
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    await expect(fetch(`${origin}/api/v1/isolation`)).rejects.toThrow()
    expect(assertNoUnhandledRequests).toThrow(/api\/v1\/isolation/)
  })
  it('delivers user clicks to a shadcn button', async () => {
    const clicked = vi.fn()
    render(<Button onClick={clicked}>Continuar</Button>)
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Continuar' }))
    expect(clicked).toHaveBeenCalledOnce()
  })
  it('opens a Radix menu with the keyboard', async () => {
    render(
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button>Acciones</Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem>Editar</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>,
    )
    const user = userEvent.setup()
    await user.tab()
    await user.keyboard('{Enter}')
    expect(
      await screen.findByRole('menuitem', { name: 'Editar' }),
    ).toBeVisible()
  })
})
