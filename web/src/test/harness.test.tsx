import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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

const origin = window.location.origin

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
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    await expect(
      fetch(
        new Request(`${origin}/api/v1/missing-handler`, { method: 'POST' }),
      ),
    ).rejects.toThrow()
    expect(log.mock.calls.flat().join(' ')).toContain('POST')
    expect(log.mock.calls.flat().join(' ')).toContain('/api/v1/missing-handler')
  })
  it('can populate a test cache, override a handler and save a temporary flag', () => {
    createTestQueryClient().setQueryData(['session'], {
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
    expect(createTestQueryClient().getQueryData(['session'])).toBeUndefined()
    expect(sessionStorage.length).toBe(0)
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    await expect(fetch(`${origin}/api/v1/isolation`)).rejects.toThrow()
    expect(log).toHaveBeenCalled()
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
