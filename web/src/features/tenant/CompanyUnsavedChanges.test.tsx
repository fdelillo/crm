import { act, screen, waitFor, within } from '@testing-library/react'
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
import { queryKeys } from '@/api/queryKeys'
import { buildCurrentUser, buildSessionInfo } from '@/test/fixtures'
import {
  base,
  companyTenant,
  preloadCompanyModules,
  problemResponse,
  serveCompany,
} from '@/test/company'
import { jpegBytes } from '@/test/imageBytes'
import { nodeBlob, nodeFile, useNodeMultipart } from '@/test/nodeFiles'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import { decodeImage, encodeBitmap } from './logo/canvas'

vi.mock('./logo/canvas', () => ({
  decodeImage: vi.fn(),
  encodeBitmap: vi.fn(),
}))

const expiredLine = 'Tu sesión se cerró. Ingresá de nuevo para seguir.'
const lostLine = 'Los cambios que no habías guardado se perdieron.'
const promptTitle = 'Tenés cambios sin guardar. ¿Salir igual?'

function sessionExpires() {
  server.use(
    http.get(`${base}/me`, () => problemResponse('unauthenticated', 401)),
  )
}
async function renderCompany(path = '/settings/company') {
  const view = renderApp([path])
  await screen.findByRole('textbox', { name: 'Teléfono' })
  return view
}
async function editPhone() {
  const user = userEvent.setup()
  await user.type(screen.getByRole('textbox', { name: 'Teléfono' }), '1')
  return user
}
const expiredPath = '/login?next=%2Fsettings%2Fcompany&reason=session_expired'
const where = (router: {
  state: { location: { pathname: string; search: string } }
}) => router.state.location.pathname + router.state.location.search

// T-F602 (BR-F10, INV-F18, DD-F32, S-F13, ui.md §12.4, §13.11)
describe('CompanyPage unsaved changes and expired sessions', () => {
  beforeAll(preloadCompanyModules)
  beforeEach(useNodeMultipart)
  afterEach(() => vi.unstubAllGlobals())

  it('asks before leaving with changes: "Quedarme" stays, "Salir" goes', async () => {
    serveCompany()
    const { router } = await renderCompany()
    const user = await editPhone()
    await user.click(screen.getByRole('link', { name: '← Ajustes' }))
    const dialog = await screen.findByRole('alertdialog', { name: promptTitle })
    // T-F602 (P-F21): the text is true for a form, a logo being prepared and a logo being uploaded.
    expect(dialog).toHaveAccessibleDescription(
      'Si salís ahora, se pierden los cambios que todavía no se guardaron.',
    )
    expect(
      within(dialog).getByRole('button', { name: 'Quedarme' }),
    ).toHaveFocus()
    await user.click(within(dialog).getByRole('button', { name: 'Quedarme' }))
    expect(router.state.location.pathname).toBe('/settings/company')
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(screen.getByRole('textbox', { name: 'Teléfono' })).toHaveValue(
      '11 5555-00001',
    )
    await user.click(screen.getByRole('link', { name: '← Ajustes' }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Salir',
      }),
    )
    expect(
      await screen.findByRole('heading', { name: 'Ajustes' }),
    ).toBeVisible()
    expect(router.state.location.pathname).toBe('/settings')
  })

  it('does not ask when nothing changed', async () => {
    serveCompany()
    const { router } = await renderCompany()
    await userEvent.click(screen.getByRole('link', { name: '← Ajustes' }))
    expect(
      await screen.findByRole('heading', { name: 'Ajustes' }),
    ).toBeVisible()
    expect(router.state.location.pathname).toBe('/settings')
  })

  it('does not ask after saving', async () => {
    serveCompany()
    await renderCompany()
    const user = await editPhone()
    await user.click(screen.getByRole('button', { name: 'Guardar cambios' }))
    await screen.findByText('Guardamos los datos de la empresa.')
    await user.click(screen.getByRole('link', { name: '← Ajustes' }))
    expect(
      await screen.findByRole('heading', { name: 'Ajustes' }),
    ).toBeVisible()
  })

  it('goes to the login without the prompt when saving gets a 401 and says the changes were lost', async () => {
    serveCompany()
    server.use(
      http.patch(`${base}/tenant`, () => {
        sessionExpires()
        return problemResponse('unauthenticated', 401)
      }),
    )
    const { router } = await renderCompany()
    const user = await editPhone()
    await user.click(screen.getByRole('button', { name: 'Guardar cambios' }))
    expect(await screen.findByText(expiredLine)).toBeVisible()
    expect(screen.getByText(lostLine)).toBeVisible()
    expect(where(router)).toBe(expiredPath)
    expect(screen.queryByText(promptTitle)).toBeNull()
  })

  it('goes to the login the same way when the session turns null on refocus', async () => {
    serveCompany()
    const { router, queryClient } = await renderCompany()
    await editPhone()
    sessionExpires()
    await act(() =>
      queryClient.invalidateQueries({ queryKey: queryKeys.session }),
    )
    expect(await screen.findByText(expiredLine)).toBeVisible()
    expect(screen.getByText(lostLine)).toBeVisible()
    expect(where(router)).toBe(expiredPath)
  })

  it('does not upload a logo that was still being prepared when the session expired', async () => {
    serveCompany()
    const put = vi.fn()
    server.use(http.put(`${base}/tenant/logo`, put))
    let finishDecoding!: () => void
    const decoding = new Promise<void>((resolve) => {
      finishDecoding = resolve
    })
    vi.mocked(decodeImage).mockImplementation(async () => {
      await decoding
      return {
        width: 4000,
        height: 3000,
        close: vi.fn(),
      } as unknown as ImageBitmap
    })
    vi.mocked(encodeBitmap).mockResolvedValue(nodeBlob(1000))
    const { router, queryClient } = await renderCompany()
    await userEvent.upload(
      screen.getByLabelText('Cambiar logo'),
      nodeFile(
        jpegBytes({ width: 4000, height: 3000, size: 5_000_000 }),
        'foto.jpg',
        'image/jpeg',
      ),
    )
    await screen.findByText('Preparando la imagen…')
    sessionExpires()
    await act(() =>
      queryClient.invalidateQueries({ queryKey: queryKeys.session }),
    )
    expect(await screen.findByText(expiredLine)).toBeVisible()
    expect(screen.getByText(lostLine)).toBeVisible()
    expect(where(router)).toBe(expiredPath)
    await act(async () => {
      finishDecoding()
      await new Promise((resolve) => setTimeout(resolve, 50))
    })
    expect(put).not.toHaveBeenCalled()
    // prepareLogo cannot be cancelled and keeps going; the page just never sends the result.
    expect(vi.mocked(encodeBitmap)).toHaveBeenCalled()
  })

  it('closes the open prompt when the 401 arrives, and the login returns to /settings/company', async () => {
    serveCompany()
    const { router, queryClient } = await renderCompany()
    const user = await editPhone()
    await user.click(screen.getByRole('link', { name: '← Ajustes' }))
    expect(
      await screen.findByRole('alertdialog', { name: promptTitle }),
    ).toBeVisible()
    sessionExpires()
    await act(() =>
      queryClient.invalidateQueries({ queryKey: queryKeys.session }),
    )
    expect(await screen.findByText(expiredLine)).toBeVisible()
    expect(screen.getByText(lostLine)).toBeVisible()
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(where(router)).toBe(expiredPath)
  })

  it('comes back to the saved data after logging in again, with nothing to save', async () => {
    serveCompany(companyTenant({ phone: '11 5555-0000' }))
    const { router, queryClient } = await renderCompany()
    const user = await editPhone()
    sessionExpires()
    await act(() =>
      queryClient.invalidateQueries({ queryKey: queryKeys.session }),
    )
    await screen.findByText(lostLine)
    serveCompany(companyTenant({ phone: '11 5555-0000' }))
    server.use(
      http.post(`${base}/auth/login`, () =>
        HttpResponse.json(
          buildSessionInfo({
            user: buildCurrentUser({ email_verified: true }),
          }),
        ),
      ),
    )
    await user.type(
      await screen.findByLabelText('Email'),
      'persona@example.test',
    )
    await user.type(screen.getByLabelText('Contraseña'), 'clave-segura')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(
      await screen.findByRole('textbox', { name: 'Teléfono' }),
    ).toHaveValue('11 5555-0000')
    expect(router.state.location.pathname).toBe('/settings/company')
    expect(
      screen.getByRole('button', { name: 'Guardar cambios' }),
    ).toBeDisabled()
    expect(queryClient.getQueryData(queryKeys.session)).toBeTruthy()
  })

  it('shows only the first line when there was nothing unsaved', async () => {
    serveCompany()
    const { queryClient } = await renderCompany()
    sessionExpires()
    await act(() =>
      queryClient.invalidateQueries({ queryKey: queryKeys.session }),
    )
    expect(await screen.findByText(expiredLine)).toBeVisible()
    expect(screen.queryByText(lostLine)).toBeNull()
  })

  it('leaves no guard behind: after "Salir", a 401 elsewhere has no second line', async () => {
    serveCompany()
    const { router, queryClient } = await renderCompany()
    const user = await editPhone()
    await user.click(screen.getByRole('link', { name: '← Ajustes' }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Salir',
      }),
    )
    await screen.findByRole('heading', { name: 'Ajustes' })
    sessionExpires()
    await act(() =>
      queryClient.invalidateQueries({ queryKey: queryKeys.session }),
    )
    expect(await screen.findByText(expiredLine)).toBeVisible()
    expect(screen.queryByText(lostLine)).toBeNull()
    expect(where(router)).toBe('/login?next=%2Fsettings&reason=session_expired')
  })
})
