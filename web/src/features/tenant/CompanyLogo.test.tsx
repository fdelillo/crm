import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from 'vitest'
import type { Tenant } from '@/api/types'
import {
  base,
  companyTenant,
  preloadCompanyModules,
  problemResponse,
  serveCompany,
} from '@/test/company'
import {
  gifBytes,
  jpegBytes,
  pngBytes,
  textBytes,
  webpBytes,
} from '@/test/imageBytes'
import {
  nodeBlob,
  nodeFile,
  sameBytes,
  useNodeMultipart,
} from '@/test/nodeFiles'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { decodeImage, encodeBitmap } from './logo/canvas'

// The canvas adapter is replaced: jsdom has no createImageBitmap nor canvas (ui.md §13.11.1).
vi.mock('./logo/canvas', () => ({
  decodeImage: vi.fn(),
  encodeBitmap: vi.fn(),
}))

const uploadedAt = '2026-10-16T10:00:00Z'
const withLogo = () => companyTenant({ has_logo: true })
const srcFor = (tenant: Pick<Tenant, 'id'>, updatedAt: string) =>
  `/api/v1/tenant/logo?v=${encodeURIComponent(`${tenant.id}-${updatedAt}`)}`
const bitmap = () =>
  ({ width: 4000, height: 3000, close: vi.fn() }) as unknown as ImageBitmap
function gate() {
  let open!: () => void
  const opened = new Promise<void>((resolve) => {
    open = resolve
  })
  return { opened, open }
}

interface Put {
  parts: string[]
  bytes: Uint8Array
  type: string
}
/** PUT/DELETE /tenant/logo against the state of serveCompany; `puts` records the multipart. */
function serveLogo(state: ReturnType<typeof serveCompany>) {
  const puts: Put[] = []
  server.use(
    http.put(`${base}/tenant/logo`, async ({ request }) => {
      const form = await request.formData()
      const file = form.get('file') as File
      puts.push({
        parts: [...(form as unknown as Iterable<[string, unknown]>)].map(
          ([key]) => key,
        ),
        bytes: new Uint8Array(await file.arrayBuffer()),
        type: file.type,
      })
      state.tenant = { ...state.tenant, has_logo: true, updated_at: uploadedAt }
      return HttpResponse.json(state.tenant)
    }),
    http.delete(`${base}/tenant/logo`, () => {
      state.tenant = {
        ...state.tenant,
        has_logo: false,
        updated_at: uploadedAt,
      }
      return new HttpResponse(null, { status: 204 })
    }),
  )
  return puts
}

async function renderCompany() {
  const view = renderApp(['/settings/company'])
  await screen.findByRole('heading', { name: 'Datos de la empresa' })
  await screen.findByRole('textbox', { name: 'Razón social' })
  return view
}
const section = () => screen.getByRole('region', { name: 'Logo' })
const picker = () => screen.getByLabelText('Cambiar logo') as HTMLInputElement
const logoImages = () => screen.queryAllByRole('img', { name: /Logo de/ })
const smallPng = () =>
  nodeFile(pngBytes(800, 400, 300_000), 'logo.png', 'image/png')
const bigJpeg = () =>
  nodeFile(
    jpegBytes({ width: 4000, height: 3000, exifBytes: 1000, size: 5_000_000 }),
    'foto.jpg',
    'image/jpeg',
  )

// T-F603 (US-4, DD-11, DD-23, DD-31, DD-F11, DD-F12, DD-F22, BR-F13, INV-F14, ui.md §11.1, §13.11, §14.4)
describe('company logo', () => {
  beforeAll(preloadCompanyModules)
  beforeEach(useNodeMultipart)
  afterEach(() => vi.unstubAllGlobals())

  it('shows initials and the empty note when there is no logo', async () => {
    serveCompany(companyTenant())
    await renderCompany()
    expect(logoImages()).toHaveLength(0)
    expect(screen.getByText('Todavía no subiste un logo')).toBeVisible()
    expect(screen.getAllByText('ED')).toHaveLength(2)
    expect(
      within(section()).getByRole('button', { name: 'Quitar' }),
    ).toBeDisabled()
  })

  it('shows the versioned logo URL on the screen and in the header (INV-F14)', async () => {
    const state = serveCompany(withLogo())
    await renderCompany()
    const images = await screen.findAllByRole('img', {
      name: 'Logo de Empresa de prueba',
    })
    expect(images).toHaveLength(2)
    for (const image of images)
      expect(image).toHaveAttribute(
        'src',
        srcFor(state.tenant, state.tenant.updated_at),
      )
    expect(screen.queryByText('Todavía no subiste un logo')).toBeNull()
  })

  it('uploads a small PNG as is, in a single "file" part', async () => {
    const state = serveCompany(companyTenant())
    const puts = serveLogo(state)
    await renderCompany()
    const file = smallPng()
    await userEvent.setup().upload(picker(), file)
    expect(await screen.findByText('Logo actualizado.')).toBeVisible()
    expect(puts).toHaveLength(1)
    expect(puts[0]!.parts).toEqual(['file'])
    expect(puts[0]!.type).toBe('image/png')
    expect(
      sameBytes(puts[0]!.bytes, new Uint8Array(await file.arrayBuffer())),
    ).toBe(true)
    expect(vi.mocked(decodeImage)).not.toHaveBeenCalled()
    expect(vi.mocked(encodeBitmap)).not.toHaveBeenCalled()
    expect(within(section()).getByRole('status')).toBeEmptyDOMElement()
    expect(await screen.findAllByRole('img', { name: /Logo de/ })).toHaveLength(
      2,
    )
  })

  it('uploads a 1000x1000 PNG of exactly 2 097 152 bytes untouched', async () => {
    const state = serveCompany(companyTenant())
    const puts = serveLogo(state)
    await renderCompany()
    const file = nodeFile(
      pngBytes(1000, 1000, 2_097_152),
      'logo.png',
      'image/png',
    )
    await userEvent.setup().upload(picker(), file)
    await screen.findByText('Logo actualizado.')
    expect(puts[0]!.bytes.length).toBe(2_097_152)
    expect(
      sameBytes(puts[0]!.bytes, new Uint8Array(await file.arrayBuffer())),
    ).toBe(true)
    expect(vi.mocked(decodeImage)).not.toHaveBeenCalled()
  })

  it('prepares a big JPEG with a live region that was already there, then uploads it', async () => {
    const state = serveCompany(withLogo())
    const puts = serveLogo(state)
    const decoding = gate()
    const uploading = gate()
    vi.mocked(decodeImage).mockImplementation(async () => {
      await decoding.opened
      return bitmap()
    })
    vi.mocked(encodeBitmap).mockResolvedValue(nodeBlob(1_500_000))
    server.use(
      http.put(`${base}/tenant/logo`, async () => {
        await uploading.opened
        state.tenant = {
          ...state.tenant,
          has_logo: true,
          updated_at: uploadedAt,
        }
        return HttpResponse.json(state.tenant)
      }),
    )
    await renderCompany()
    const region = within(section()).getByRole('status')
    expect(region).toBeEmptyDOMElement()
    await userEvent.setup().upload(picker(), bigJpeg())
    await waitFor(() =>
      expect(region).toHaveTextContent('Preparando la imagen…'),
    )
    expect(within(section()).getByRole('status')).toBe(region)
    expect(picker()).toBeDisabled()
    expect(
      within(section()).getByRole('button', { name: 'Quitar' }),
    ).toBeDisabled()
    decoding.open()
    await waitFor(() => expect(region).toHaveTextContent('Subiendo logo…'))
    expect(picker()).toBeDisabled()
    uploading.open()
    expect(
      await screen.findByText(
        'Logo actualizado. Lo achicamos para que pese menos.',
      ),
    ).toBeVisible()
    expect(region).toBeEmptyDOMElement()
    expect(picker()).toBeEnabled()
    for (const image of await screen.findAllByRole('img', { name: /Logo de/ }))
      expect(image).toHaveAttribute('src', srcFor(state.tenant, uploadedAt))
    expect(puts).toHaveLength(0)
  })

  it('sends a JPEG of at most 2 097 152 bytes after preparing a big photo', async () => {
    const state = serveCompany(companyTenant())
    const puts = serveLogo(state)
    vi.mocked(decodeImage).mockResolvedValue(bitmap())
    vi.mocked(encodeBitmap).mockResolvedValue(nodeBlob(1_500_000))
    await renderCompany()
    await userEvent.setup().upload(picker(), bigJpeg())
    await screen.findByText(
      'Logo actualizado. Lo achicamos para que pese menos.',
    )
    expect(puts).toHaveLength(1)
    expect(puts[0]!.type).toBe('image/jpeg')
    expect(puts[0]!.bytes.length).toBeLessThanOrEqual(2_097_152)
    expect(vi.mocked(decodeImage)).toHaveBeenCalledOnce()
  })

  it.each([
    [
      'an SVG',
      textBytes('<svg xmlns="http://www.w3.org/2000/svg"></svg>'),
      'No podemos usar logos en SVG. Exportalo como PNG desde el programa donde lo tenés y subilo de nuevo.',
    ],
    ['a GIF', gifBytes(), 'El logo tiene que ser una imagen PNG o JPG.'],
    ['a WebP', webpBytes(), 'El logo tiene que ser una imagen PNG o JPG.'],
    [
      'bytes of no known image',
      textBytes('esto no es una imagen'),
      'El logo tiene que ser una imagen PNG o JPG.',
    ],
  ])(
    'rejects %s in the logo control without any request',
    async (_name, bytes, message) => {
      const state = serveCompany(companyTenant())
      const puts = serveLogo(state)
      await renderCompany()
      await userEvent
        .setup({ applyAccept: false })
        .upload(
          picker(),
          nodeFile(
            bytes as Uint8Array<ArrayBuffer>,
            'logo.bin',
            'application/octet-stream',
          ),
        )
      expect(await within(section()).findByRole('alert')).toHaveTextContent(
        message,
      )
      expect(puts).toHaveLength(0)
      expect(vi.mocked(decodeImage)).not.toHaveBeenCalled()
    },
  )

  it('reports an unreadable image when the PNG header is cut', async () => {
    const state = serveCompany(companyTenant())
    const puts = serveLogo(state)
    await renderCompany()
    await userEvent
      .setup()
      .upload(
        picker(),
        nodeFile(pngBytes(10, 10).slice(0, 12), 'logo.png', 'image/png'),
      )
    expect(await within(section()).findByRole('alert')).toHaveTextContent(
      'No pudimos leer la imagen. Probá con otro archivo PNG o JPG.',
    )
    expect(puts).toHaveLength(0)
  })

  it('reports an unreadable image when the adapter fails to decode', async () => {
    const state = serveCompany(companyTenant())
    const puts = serveLogo(state)
    vi.mocked(decodeImage).mockRejectedValue(new Error('decode'))
    await renderCompany()
    await userEvent.setup().upload(picker(), bigJpeg())
    expect(await within(section()).findByRole('alert')).toHaveTextContent(
      'No pudimos leer la imagen. Probá con otro archivo PNG o JPG.',
    )
    expect(puts).toHaveLength(0)
    expect(picker()).toBeEnabled()
  })

  it('reports it when no step of the ladder fits', async () => {
    const state = serveCompany(companyTenant())
    const puts = serveLogo(state)
    vi.mocked(decodeImage).mockResolvedValue(bitmap())
    vi.mocked(encodeBitmap).mockResolvedValue(nodeBlob(3_000_000))
    await renderCompany()
    await userEvent.setup().upload(picker(), bigJpeg())
    expect(await within(section()).findByRole('alert')).toHaveTextContent(
      'No pudimos achicar la imagen lo suficiente. Probá con otra imagen del logo.',
    )
    expect(puts).toHaveLength(0)
  })

  it.each([
    [
      413,
      'payload_too_large',
      'La imagen sigue pesando más de lo permitido. Probá con otra imagen del logo.',
    ],
    [
      415,
      'unsupported_media_type',
      'El logo tiene que ser una imagen PNG o JPG.',
    ],
  ] as const)(
    'shows the server %i in the logo control',
    async (status, code, message) => {
      serveCompany(companyTenant())
      server.use(
        http.put(`${base}/tenant/logo`, () => problemResponse(code, status)),
      )
      await renderCompany()
      await userEvent.setup().upload(picker(), smallPng())
      expect(await within(section()).findByRole('alert')).toHaveTextContent(
        message,
      )
    },
  )

  it.each([
    [
      'invalid_value',
      'No pudimos leer la imagen o mide más de 2000 × 2000 px. Probá con otra imagen del logo.',
    ],
    ['required', 'Elegí una imagen para el logo.'],
  ] as const)(
    'shows the 422 file/%s in the logo control, not in a form field',
    async (code, message) => {
      serveCompany(companyTenant())
      server.use(
        http.put(`${base}/tenant/logo`, () =>
          problemResponse('validation_failed', 422, {
            errors: [{ field: 'file', code }],
          }),
        ),
      )
      await renderCompany()
      await userEvent.setup().upload(picker(), smallPng())
      expect(await within(section()).findByRole('alert')).toHaveTextContent(
        message,
      )
      expect(screen.getAllByRole('alert')).toHaveLength(1)
      expect(screen.queryByRole('button', { name: 'Reintentar' })).toBeNull()
    },
  )

  it('tells the client bug apart for a 400, with the code', async () => {
    serveCompany(companyTenant())
    server.use(
      http.put(`${base}/tenant/logo`, () =>
        problemResponse('malformed_request', 400, { instance: 'req-9' }),
      ),
    )
    await renderCompany()
    await userEvent.setup().upload(picker(), smallPng())
    const alert = await within(section()).findByRole('alert')
    expect(alert).toHaveTextContent('No pudimos subir el logo.')
    expect(alert).toHaveTextContent('Recargá la página y probá de nuevo.')
    expect(alert).toHaveTextContent('req-9')
  })

  it.each([
    ['without a previous logo', companyTenant()],
    ['replacing a previous logo', withLogo()],
  ])(
    'offers "Reintentar" after a 503 when the logo was not saved, %s, and retrying works',
    async (_name, initial) => {
      const state = serveCompany(initial)
      let attempts = 0
      server.use(
        http.put(`${base}/tenant/logo`, () => {
          attempts += 1
          if (attempts === 1) return problemResponse('service_unavailable', 503)
          state.tenant = {
            ...state.tenant,
            has_logo: true,
            updated_at: uploadedAt,
          }
          return HttpResponse.json(state.tenant)
        }),
      )
      await renderCompany()
      await userEvent.setup().upload(picker(), smallPng())
      const alert = await within(section()).findByRole('alert')
      expect(alert).toHaveTextContent(
        'El servicio no está disponible en este momento.',
      )
      await userEvent.click(
        within(alert).getByRole('button', { name: 'Reintentar' }),
      )
      expect(await screen.findByText('Logo actualizado.')).toBeVisible()
      expect(attempts).toBe(2)
      expect(within(section()).queryByRole('alert')).toBeNull()
    },
  )

  it('offers "Reintentar" without a connection too', async () => {
    serveCompany(companyTenant())
    server.use(http.put(`${base}/tenant/logo`, () => HttpResponse.error()))
    await renderCompany()
    await userEvent.setup().upload(picker(), smallPng())
    const alert = await within(section()).findByRole('alert')
    expect(alert).toHaveTextContent('No hay conexión.')
    expect(
      within(alert).getByRole('button', { name: 'Reintentar' }),
    ).toBeVisible()
  })

  it('treats a 503 as success when the tenant read afterwards shows the new logo (DD-F26)', async () => {
    const state = serveCompany(withLogo())
    server.use(
      http.put(`${base}/tenant/logo`, () => {
        state.tenant = {
          ...state.tenant,
          has_logo: true,
          updated_at: uploadedAt,
        }
        return problemResponse('service_unavailable', 503)
      }),
    )
    await renderCompany()
    await userEvent.setup().upload(picker(), smallPng())
    expect(await screen.findByText('Logo actualizado.')).toBeVisible()
    expect(within(section()).queryByRole('alert')).toBeNull()
    await waitFor(() => {
      for (const image of logoImages())
        expect(image).toHaveAttribute('src', srcFor(state.tenant, uploadedAt))
    })
    expect(logoImages()).toHaveLength(2)
  })

  it('treats a lost response as success when the tenant read afterwards shows the new logo', async () => {
    const state = serveCompany(companyTenant())
    server.use(
      http.put(`${base}/tenant/logo`, () => {
        state.tenant = {
          ...state.tenant,
          has_logo: true,
          updated_at: uploadedAt,
        }
        return HttpResponse.error()
      }),
    )
    await renderCompany()
    await userEvent.setup().upload(picker(), smallPng())
    expect(await screen.findByText('Logo actualizado.')).toBeVisible()
    expect(within(section()).queryByRole('alert')).toBeNull()
  })

  it('removes the logo after confirming and says so', async () => {
    const state = serveCompany(withLogo())
    serveLogo(state)
    await renderCompany()
    const user = userEvent.setup()
    await user.click(within(section()).getByRole('button', { name: 'Quitar' }))
    const dialog = await screen.findByRole('alertdialog', {
      name: '¿Quitar el logo?',
    })
    await user.click(within(dialog).getByRole('button', { name: 'Quitar' }))
    expect(await screen.findByText('Quitamos el logo.')).toBeVisible()
    await waitFor(() => expect(logoImages()).toHaveLength(0))
    expect(screen.getByText('Todavía no subiste un logo')).toBeVisible()
    expect(screen.getAllByText('ED')).toHaveLength(2)
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(picker()).toHaveFocus()
  })

  it('keeps the logo and the focus on "Quitar" when the removal is cancelled', async () => {
    const state = serveCompany(withLogo())
    const deleted = vi.fn()
    server.use(http.delete(`${base}/tenant/logo`, deleted))
    await renderCompany()
    const user = userEvent.setup()
    const remove = within(section()).getByRole('button', { name: 'Quitar' })
    await user.click(remove)
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Cancelar',
      }),
    )
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(deleted).not.toHaveBeenCalled()
    expect(logoImages()).toHaveLength(2)
    expect(remove).toHaveFocus()
    expect(state.tenant.has_logo).toBe(true)
  })

  it('treats a 503 on removal as done when the tenant read afterwards has no logo (DD-F26)', async () => {
    const state = serveCompany(withLogo())
    server.use(
      http.delete(`${base}/tenant/logo`, () => {
        state.tenant = {
          ...state.tenant,
          has_logo: false,
          updated_at: uploadedAt,
        }
        return problemResponse('service_unavailable', 503)
      }),
    )
    await renderCompany()
    const user = userEvent.setup()
    await user.click(within(section()).getByRole('button', { name: 'Quitar' }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Quitar',
      }),
    )
    expect(await screen.findByText('Quitamos el logo.')).toBeVisible()
    await waitFor(() => expect(logoImages()).toHaveLength(0))
    expect(within(section()).queryByRole('alert')).toBeNull()
  })

  it('shows the error with "Reintentar" when the removal did not happen', async () => {
    serveCompany(withLogo())
    server.use(
      http.delete(`${base}/tenant/logo`, () =>
        problemResponse('service_unavailable', 503),
      ),
    )
    await renderCompany()
    const user = userEvent.setup()
    await user.click(within(section()).getByRole('button', { name: 'Quitar' }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Quitar',
      }),
    )
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    const alert = await within(section()).findByRole('alert')
    expect(
      within(alert).getByRole('button', { name: 'Reintentar' }),
    ).toBeVisible()
    expect(logoImages()).toHaveLength(2)
  })

  it('asks before leaving while the logo is being prepared (BR-F10)', async () => {
    const state = serveCompany(companyTenant())
    serveLogo(state)
    const decoding = gate()
    vi.mocked(decodeImage).mockImplementation(async () => {
      await decoding.opened
      return bitmap()
    })
    vi.mocked(encodeBitmap).mockResolvedValue(nodeBlob(1000))
    const { router } = await renderCompany()
    const user = userEvent.setup()
    await user.upload(picker(), bigJpeg())
    await waitFor(() =>
      expect(within(section()).getByRole('status')).toHaveTextContent(
        'Preparando la imagen…',
      ),
    )
    await user.click(screen.getByRole('link', { name: '← Ajustes' }))
    expect(
      await screen.findByRole('alertdialog', {
        name: 'Tenés cambios sin guardar. ¿Salir igual?',
      }),
    ).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Quedarme' }))
    expect(router.state.location.pathname).toBe('/settings/company')
    decoding.open()
    await screen.findByText(
      'Logo actualizado. Lo achicamos para que pese menos.',
    )
  })

  it('falls back to the initials, without any message, when the image does not load', async () => {
    serveCompany(withLogo())
    await renderCompany()
    for (const image of await screen.findAllByRole('img', { name: /Logo de/ }))
      fireEvent.error(image)
    await waitFor(() => expect(logoImages()).toHaveLength(0))
    expect(screen.getAllByText('ED')).toHaveLength(2)
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('has an accessible, keyboard-reachable file control that accepts PNG and JPEG', async () => {
    serveCompany(companyTenant())
    await renderCompany()
    expect(picker()).toHaveAttribute('accept', 'image/png,image/jpeg')
    expect(picker()).toHaveAttribute('type', 'file')
    const user = userEvent.setup()
    for (let tabs = 0; tabs < 8 && document.activeElement !== picker(); tabs++)
      await user.tab()
    expect(picker()).toHaveFocus()
  })
})
