import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Tenant, TenantUpdate } from '@/api/types'
import {
  base,
  companyTenant,
  preloadCompanyModules,
  problemResponse,
} from '@/test/company'
import { buildCurrentUser, buildSessionInfo } from '@/test/fixtures'
import { gate } from '@/test/gate'
import { pngBytes } from '@/test/imageBytes'
import { nodeFile, useNodeMultipart } from '@/test/nodeFiles'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'

// The canvas adapter is not exercised (the PNG is small enough to go as it is) but jsdom has none.
vi.mock('./logo/canvas', () => ({
  decodeImage: vi.fn(),
  encodeBitmap: vi.fn(),
}))

type Gate = ReturnType<typeof gate>
const srcFor = (tenant: Pick<Tenant, 'id'>, updatedAt: string) =>
  `/api/v1/tenant/logo?v=${encodeURIComponent(`${tenant.id}-${updatedAt}`)}`

/**
 * A server where every request commits when it is RECEIVED (the lock of the row, then the COMMIT)
 * and answers with a photo of the row at that moment; a gate holds the answer, which is how the
 * tests reorder responses. `events` is the order in which the server saw the requests.
 */
function serveTenant(initial: Tenant) {
  const events: string[] = []
  const state = { tenant: initial, clock: 0, patches: [] as TenantUpdate[] }
  const holds: { put?: Gate; patch?: Gate; delete?: Gate; read?: Gate } = {}
  const rejects: { put?: () => Response } = {}
  const stamp = () => `2026-10-16T10:00:0${++state.clock}Z`
  server.use(
    http.get(`${base}/me`, () =>
      HttpResponse.json(
        buildSessionInfo({ user: buildCurrentUser({ email_verified: true }) }),
      ),
    ),
    http.get(`${base}/tenant`, async () => {
      const photo = state.tenant
      if (holds.read) {
        events.push('GET retenido')
        await holds.read.opened
      }
      return HttpResponse.json(photo)
    }),
    http.patch(`${base}/tenant`, async ({ request }) => {
      const body = (await request.json()) as TenantUpdate
      state.patches.push(body)
      events.push('PATCH recibido')
      state.tenant = { ...state.tenant, ...body, updated_at: stamp() } as Tenant
      const photo = state.tenant
      await holds.patch?.opened
      return HttpResponse.json(photo)
    }),
    http.put(`${base}/tenant/logo`, async () => {
      events.push('PUT recibido')
      const rejected = rejects.put
      if (!rejected)
        state.tenant = { ...state.tenant, has_logo: true, updated_at: stamp() }
      const photo = state.tenant
      await holds.put?.opened
      return rejected ? rejected() : HttpResponse.json(photo)
    }),
    http.delete(`${base}/tenant/logo`, async () => {
      events.push('DELETE recibido')
      state.tenant = { ...state.tenant, has_logo: false, updated_at: stamp() }
      await holds.delete?.opened
      return new HttpResponse(null, { status: 204 })
    }),
  )
  return { events, state, holds, rejects }
}

// A few turns of the event loop: enough for a request that was going to leave to reach the server.
const settle = () =>
  act(() => new Promise<void>((resolve) => setTimeout(resolve, 60)))

async function renderCompany() {
  const view = renderApp(['/settings/company'])
  await screen.findByRole('heading', { name: 'Datos de la empresa' })
  await screen.findByRole('textbox', { name: 'Teléfono' })
  return view
}
const section = () => screen.getByRole('region', { name: 'Logo' })
const picker = () => screen.getByLabelText('Cambiar logo') as HTMLInputElement
const smallPng = () =>
  nodeFile(pngBytes(800, 400, 300_000), 'logo.png', 'image/png')
const savePhone = async () => {
  const user = userEvent.setup()
  await user.type(screen.getByRole('textbox', { name: 'Teléfono' }), '1')
  await user.click(screen.getByRole('button', { name: 'Guardar cambios' }))
}
const logoSources = async () =>
  (await screen.findAllByRole('img', { name: /Logo de/ })).map((image) =>
    image.getAttribute('src'),
  )
const expectLogoSources = (tenant: Tenant, updatedAt: string) =>
  waitFor(async () =>
    expect(await logoSources()).toEqual(
      Array(2).fill(srcFor(tenant, updatedAt)),
    ),
  )
const withLogo = () => companyTenant({ has_logo: true })

// T-F602, T-F603 (DD-F33, INV-F20, S-F15, ui.md §13.11, §14.4): the changes of the company
// leave one at a time, in the order they were asked for, and a stale read never overwrites them.
describe('company changes are ordered', () => {
  beforeAll(preloadCompanyModules)
  beforeEach(useNodeMultipart)

  it('holds the PATCH until the PUT answered, and ends with the newest response', async () => {
    const backend = serveTenant(withLogo())
    backend.holds.put = gate()
    await renderCompany()
    await userEvent.setup().upload(picker(), smallPng())
    await waitFor(() => expect(backend.events).toEqual(['PUT recibido']))
    await savePhone()
    const saving = await screen.findByRole('button', { name: 'Guardando…' })
    expect(saving).toBeDisabled()
    expect(screen.getByRole('textbox', { name: 'Teléfono' })).toHaveAttribute(
      'readonly',
    )
    await settle()
    expect(backend.events).toEqual(['PUT recibido'])
    backend.holds.put.open()
    // Two toasts at once: sonner stacks them, so only their presence is stable.
    expect(
      await screen.findByText('Guardamos los datos de la empresa.'),
    ).toBeInTheDocument()
    expect(await screen.findByText('Logo actualizado.')).toBeInTheDocument()
    expect(backend.events).toEqual(['PUT recibido', 'PATCH recibido'])
    expect(backend.state.patches).toEqual([{ phone: '11 5555-00001' }])
    await expectLogoSources(backend.state.tenant, '2026-10-16T10:00:02Z')
  })

  it('still sends the PATCH when the PUT answers 413, each change with its own result', async () => {
    const backend = serveTenant(withLogo())
    backend.holds.put = gate()
    backend.rejects.put = () => problemResponse('payload_too_large', 413)
    await renderCompany()
    await userEvent.setup().upload(picker(), smallPng())
    await waitFor(() => expect(backend.events).toEqual(['PUT recibido']))
    await savePhone()
    await settle()
    expect(backend.events).toEqual(['PUT recibido'])
    backend.holds.put.open()
    expect(
      await screen.findByText('Guardamos los datos de la empresa.'),
    ).toBeVisible()
    expect(await within(section()).findByRole('alert')).toHaveTextContent(
      'La imagen sigue pesando más de lo permitido.',
    )
    expect(backend.events).toEqual(['PUT recibido', 'PATCH recibido'])
  })

  it('sends no PATCH when the PUT answers 401: the queue is emptied with the cache (S-F15)', async () => {
    const backend = serveTenant(withLogo())
    backend.holds.put = gate()
    backend.rejects.put = () => {
      // The session is gone for everything that is read from now on.
      server.use(
        http.get(`${base}/me`, () => problemResponse('unauthenticated', 401)),
      )
      return problemResponse('unauthenticated', 401)
    }
    await renderCompany()
    await userEvent.setup().upload(picker(), smallPng())
    await waitFor(() => expect(backend.events).toEqual(['PUT recibido']))
    await savePhone()
    await settle()
    expect(backend.events).toEqual(['PUT recibido'])
    backend.holds.put.open()
    expect(
      await screen.findByText(
        'Tu sesión se cerró. Ingresá de nuevo para seguir.',
      ),
    ).toBeVisible()
    expect(
      screen.getByText('Los cambios que no habías guardado se perdieron.'),
    ).toBeVisible()
    await settle()
    expect(backend.events).toEqual(['PUT recibido'])
  })

  it('holds the PUT until the PATCH answered, and ends with the newest response', async () => {
    const backend = serveTenant(withLogo())
    backend.holds.patch = gate()
    await renderCompany()
    await savePhone()
    await waitFor(() => expect(backend.events).toEqual(['PATCH recibido']))
    await userEvent.setup().upload(picker(), smallPng())
    expect(await within(section()).findByRole('status')).toHaveTextContent(
      'Subiendo logo…',
    )
    await settle()
    expect(backend.events).toEqual(['PATCH recibido'])
    backend.holds.patch.open()
    expect(await screen.findByText('Logo actualizado.')).toBeVisible()
    expect(backend.events).toEqual(['PATCH recibido', 'PUT recibido'])
    await expectLogoSources(backend.state.tenant, '2026-10-16T10:00:02Z')
    expect(backend.state.tenant.phone).toBe('11 5555-00001')
  })

  it('holds the DELETE until the PATCH answered, then shows the initials', async () => {
    const backend = serveTenant(withLogo())
    backend.holds.patch = gate()
    await renderCompany()
    await savePhone()
    await waitFor(() => expect(backend.events).toEqual(['PATCH recibido']))
    const user = userEvent.setup()
    await user.click(within(section()).getByRole('button', { name: 'Quitar' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Quitar' }))
    await within(dialog).findByRole('button', { name: 'Quitando logo…' })
    await settle()
    expect(backend.events).toEqual(['PATCH recibido'])
    backend.holds.patch.open()
    expect(await screen.findByText('Quitamos el logo.')).toBeVisible()
    expect(backend.events).toEqual(['PATCH recibido', 'DELETE recibido'])
    await waitFor(() =>
      expect(screen.queryAllByRole('img', { name: /Logo de/ })).toHaveLength(0),
    )
    expect(screen.getByText('Todavía no subiste un logo')).toBeVisible()
  })

  // A read that left before the commit and answers after the response must not bring the row back.
  it('does not let a read in flight overwrite the PATCH response', async () => {
    const backend = serveTenant(withLogo())
    const { queryClient } = await renderCompany()
    const before = backend.state.tenant
    backend.holds.read = gate()
    await act(async () => {
      void queryClient.invalidateQueries({ queryKey: ['tenant'], exact: true })
    })
    await waitFor(() => expect(backend.events).toEqual(['GET retenido']))
    await savePhone()
    expect(
      await screen.findByText('Guardamos los datos de la empresa.'),
    ).toBeVisible()
    backend.holds.read.open()
    await settle()
    await expectLogoSources(before, '2026-10-16T10:00:01Z')
    expect(screen.getByRole('textbox', { name: 'Teléfono' })).toHaveValue(
      '11 5555-00001',
    )
  })

  it('does not let a read in flight overwrite the PUT response', async () => {
    const backend = serveTenant(withLogo())
    const { queryClient } = await renderCompany()
    const before = backend.state.tenant
    backend.holds.read = gate()
    await act(async () => {
      void queryClient.invalidateQueries({ queryKey: ['tenant'], exact: true })
    })
    await waitFor(() => expect(backend.events).toEqual(['GET retenido']))
    await userEvent.setup().upload(picker(), smallPng())
    expect(await screen.findByText('Logo actualizado.')).toBeVisible()
    backend.holds.read.open()
    await settle()
    await expectLogoSources(before, '2026-10-16T10:00:01Z')
  })
})
