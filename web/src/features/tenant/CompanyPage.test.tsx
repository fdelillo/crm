import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse, delay } from 'msw'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import {
  base,
  companyTenant,
  expectSafePatch,
  preloadCompanyModules,
  problemResponse,
  savedAt,
  serveCompany,
} from '@/test/company'

async function renderCompany() {
  const view = renderApp(['/settings/company'])
  await screen.findByRole('heading', { name: 'Datos de la empresa' })
  return view
}
const save = () => screen.getByRole('button', { name: 'Guardar cambios' })
const field = (name: string) => screen.findByRole('textbox', { name })

// T-F602 (US-4, DD-15, DD-27, BR-F09, BR-F10, INV-F04, INV-F15, DD-F24, ui.md §13.11)
describe('CompanyPage data form', () => {
  beforeAll(preloadCompanyModules)

  it('shows a skeleton while loading', async () => {
    serveCompany()
    server.use(http.get(`${base}/tenant`, () => delay('infinite')))
    renderApp(['/settings/company'])
    const skeleton = await screen.findByRole('region', {
      name: 'Cargando los datos de la empresa',
    })
    expect(skeleton).toHaveAttribute('aria-busy', 'true')
    expect(
      screen.queryByRole('button', { name: 'Guardar cambios' }),
    ).not.toBeInTheDocument()
  })

  it('offers to retry when the company cannot be loaded', async () => {
    serveCompany()
    server.use(
      http.get(`${base}/tenant`, () =>
        problemResponse('service_unavailable', 503),
      ),
    )
    renderApp(['/settings/company'])
    expect(
      await screen.findByText('No pudimos cargar los datos de tu empresa.'),
    ).toBeVisible()
    server.use(
      http.get(`${base}/tenant`, () => HttpResponse.json(companyTenant())),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(await field('Razón social')).toHaveValue('ACME S.A.')
  })

  it('fills every field and shows the base currency as read-only', async () => {
    serveCompany(companyTenant({ name: 'Mi Empresa' }))
    await renderCompany()
    expect(await field('Nombre de la empresa')).toHaveValue('Mi Empresa')
    expect(await field('Razón social')).toHaveValue('ACME S.A.')
    expect(await field('CUIT')).toHaveValue('30123456781')
    expect(await field('Dirección')).toHaveValue('Calle Falsa 123')
    expect(await field('Teléfono')).toHaveValue('11 5555-0000')
    expect(await field('Email de contacto')).toHaveValue('contacto@acme.test')
    expect(
      screen.getByRole('combobox', { name: 'Zona horaria' }),
    ).toHaveTextContent('America/Argentina/Buenos_Aires')
    expect(screen.getByText('Moneda base: Pesos (ARS)')).toBeVisible()
    expect(screen.getByText(/No se puede cambiar/)).toBeVisible()
    expect(screen.queryByRole('combobox', { name: /Moneda/ })).toBeNull()
    expect(screen.getByText('11 números, con o sin guiones.')).toBeVisible()
  })

  it('shows empty optional fields with their help', async () => {
    serveCompany(
      companyTenant({
        legal_name: null,
        tax_id: null,
        address: null,
        phone: null,
        email: null,
      }),
    )
    await renderCompany()
    expect(await field('Razón social')).toHaveValue('')
    expect(await field('CUIT')).toHaveValue('')
    expect(
      screen.getByText('Se usan en los PDF de tus presupuestos.'),
    ).toBeVisible()
  })

  it('keeps "Guardar cambios" disabled until something changes', async () => {
    serveCompany()
    await renderCompany()
    await field('Teléfono')
    expect(save()).toBeDisabled()
    const user = userEvent.setup()
    await user.type(await field('Teléfono'), '1')
    expect(save()).toBeEnabled()
    await user.type(await field('Teléfono'), '{Backspace}')
    expect(save()).toBeDisabled()
  })

  it('sends only the phone when only the phone changed', async () => {
    const server_ = serveCompany()
    await renderCompany()
    const user = userEvent.setup()
    await user.clear(await field('Teléfono'))
    await user.type(await field('Teléfono'), '  11 6666-1111 ')
    await user.click(save())
    await waitFor(() => expect(server_.patches).toHaveLength(1))
    expect(server_.patches[0]).toEqual({ phone: '11 6666-1111' })
    expectSafePatch(server_.patches[0]!)
  })

  it('sends null for a cleared optional field and the CUIT as written', async () => {
    const state = serveCompany()
    await renderCompany()
    const user = userEvent.setup()
    await user.clear(await field('Razón social'))
    await user.clear(await field('CUIT'))
    await user.type(await field('CUIT'), ' 30-12345678-1 ')
    await user.click(save())
    await waitFor(() => expect(state.patches).toHaveLength(1))
    expect(state.patches[0]).toEqual({
      legal_name: null,
      tax_id: '30-12345678-1',
    })
    expectSafePatch(state.patches[0]!)
  })

  it('blocks a CUIT with a wrong check digit without sending anything', async () => {
    const state = serveCompany()
    await renderCompany()
    const user = userEvent.setup()
    await user.clear(await field('CUIT'))
    await user.type(await field('CUIT'), '30123456782')
    await user.click(save())
    expect(
      await screen.findByText('El CUIT no es válido. Revisá los números.'),
    ).toBeVisible()
    expect(await field('CUIT')).toHaveAccessibleDescription(
      /El CUIT no es válido/,
    )
    expect(state.patches).toHaveLength(0)
  })

  it('explains the CUIT format when the digits are not 11', async () => {
    const state = serveCompany()
    await renderCompany()
    const user = userEvent.setup()
    await user.clear(await field('CUIT'))
    await user.type(await field('CUIT'), '3-0123456789-1')
    await user.click(save())
    expect(
      await screen.findByText(
        'El CUIT tiene 11 números (podés escribirlo con o sin guiones).',
      ),
    ).toBeVisible()
    expect(state.patches).toHaveLength(0)
  })

  it('shows the server field errors for the CUIT and the time zone', async () => {
    serveCompany()
    server.use(
      http.patch(`${base}/tenant`, () =>
        problemResponse('validation_failed', 422, {
          errors: [
            { field: 'tax_id', code: 'invalid_tax_id' },
            { field: 'timezone', code: 'invalid_timezone' },
          ],
        }),
      ),
    )
    await renderCompany()
    const user = userEvent.setup()
    await user.type(await field('Teléfono'), '1')
    await user.click(save())
    expect(
      await screen.findByText('El CUIT no es válido. Revisá los números.'),
    ).toBeVisible()
    expect(
      screen.getByText('Elegí una zona horaria de la lista.'),
    ).toBeVisible()
    expect(await field('CUIT')).toBeInvalid()
    expect(await field('CUIT')).toHaveFocus()
  })

  it('maps invalid_value of long optional fields and of the name per field', async () => {
    serveCompany()
    server.use(
      http.patch(`${base}/tenant`, () =>
        problemResponse('validation_failed', 422, {
          errors: [
            { field: 'legal_name', code: 'invalid_value' },
            { field: 'name', code: 'invalid_value' },
          ],
        }),
      ),
    )
    await renderCompany()
    await userEvent.type(await field('Teléfono'), '1')
    await userEvent.click(save())
    expect(
      await screen.findByText('Es demasiado largo (máximo 200 caracteres).'),
    ).toBeVisible()
    expect(
      screen.getByText('Tiene que tener entre 1 y 120 caracteres.'),
    ).toBeVisible()
    expect(await field('Razón social')).toHaveAccessibleDescription(
      'Es demasiado largo (máximo 200 caracteres).',
    )
    expect(await field('Nombre de la empresa')).toHaveAccessibleDescription(
      'Tiene que tener entre 1 y 120 caracteres.',
    )
  })

  it('refuses an empty name before any request', async () => {
    const state = serveCompany()
    await renderCompany()
    const user = userEvent.setup()
    await user.clear(await field('Nombre de la empresa'))
    await user.click(save())
    expect(await screen.findByText('Completá este campo.')).toBeVisible()
    expect(state.patches).toHaveLength(0)
    expect(await field('Nombre de la empresa')).toHaveFocus()
  })

  it('toasts, resets the form and updates the header name on success', async () => {
    const state = serveCompany()
    await renderCompany()
    const user = userEvent.setup()
    const name = await field('Nombre de la empresa')
    await user.clear(name)
    await user.type(name, 'Nombre Nuevo SRL')
    await user.click(save())
    expect(
      await screen.findByText('Guardamos los datos de la empresa.'),
    ).toBeVisible()
    expect(state.patches).toEqual([{ name: 'Nombre Nuevo SRL' }])
    expect(
      await within(screen.getAllByRole('banner')[0]!).findByText(
        'Nombre Nuevo SRL',
      ),
    ).toBeVisible()
    await waitFor(() => expect(save()).toBeDisabled())
  })

  it('disables the button and says "Guardando…" while saving', async () => {
    serveCompany()
    server.use(
      http.patch(`${base}/tenant`, async () => {
        await delay('infinite')
        return HttpResponse.json({})
      }),
    )
    await renderCompany()
    await userEvent.type(await field('Teléfono'), '1')
    await userEvent.click(save())
    expect(
      await screen.findByRole('button', { name: 'Guardando…' }),
    ).toBeDisabled()
  })

  it('keeps the values and offers "Reintentar" after a 503, and a retry is safe', async () => {
    const state = serveCompany(companyTenant({ has_logo: true }))
    let attempts = 0
    server.use(
      http.patch(`${base}/tenant`, async ({ request }) => {
        attempts += 1
        state.patches.push((await request.json()) as never)
        return attempts === 1
          ? problemResponse('service_unavailable', 503)
          : HttpResponse.json(state.tenant)
      }),
    )
    await renderCompany()
    const logo = await screen.findAllByRole('img', { name: /Logo de/ })
    const before = logo.map((image) => image.getAttribute('src'))
    const user = userEvent.setup()
    await user.clear(await field('Teléfono'))
    await user.type(await field('Teléfono'), '11 7777-2222')
    await user.click(save())
    const alert = await screen.findByRole('alert')
    expect(
      within(alert).getByText(
        'El servicio no está disponible en este momento.',
      ),
    ).toBeVisible()
    expect(await field('Teléfono')).toHaveValue('11 7777-2222')
    await user.click(within(alert).getByRole('button', { name: 'Reintentar' }))
    expect(
      await screen.findByText('Guardamos los datos de la empresa.'),
    ).toBeVisible()
    expect(state.patches).toHaveLength(2)
    expect(state.patches[1]).toEqual(state.patches[0])
    expect(
      screen
        .getAllByRole('img', { name: /Logo de/ })
        .map((image) => image.getAttribute('src')),
    ).toEqual(before)
  })

  it('shows a general error with the code for a 400 malformed_request', async () => {
    serveCompany()
    server.use(
      http.patch(`${base}/tenant`, () =>
        problemResponse('malformed_request', 400, { instance: 'req-123' }),
      ),
    )
    await renderCompany()
    await userEvent.type(await field('Teléfono'), '1')
    await userEvent.click(save())
    const alert = await screen.findByRole('alert')
    expect(
      within(alert).getByText('No pudimos procesar el pedido.'),
    ).toBeVisible()
    expect(
      within(alert).getByText('Recargá la página y probá de nuevo.'),
    ).toBeVisible()
    expect(within(alert).getByText(/req-123/)).toBeVisible()
    expect(
      within(alert).queryByRole('button', { name: 'Reintentar' }),
    ).toBeNull()
  })
})

describe('CompanyPage time zone', () => {
  beforeAll(preloadCompanyModules)
  beforeEach(() => {
    vi.spyOn(Intl, 'supportedValuesOf').mockReturnValue([
      'Africa/Abidjan',
      'America/Argentina/Buenos_Aires',
      'America/Argentina/Cordoba',
      'Europe/Madrid',
    ])
  })

  it('lists America/Argentina/* first', async () => {
    serveCompany(companyTenant({ timezone: 'Europe/Madrid' }))
    await renderCompany()
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('combobox', { name: 'Zona horaria' }),
    )
    const options = (await screen.findAllByRole('option')).map(
      (option) => option.textContent,
    )
    expect(options).toEqual([
      'America/Argentina/Buenos_Aires',
      'America/Argentina/Cordoba',
      'Africa/Abidjan',
      'Europe/Madrid',
    ])
  })

  it('shows the saved zone selected even when Intl does not list it (DD-F27)', async () => {
    serveCompany(companyTenant({ timezone: 'America/Buenos_Aires' }))
    await renderCompany()
    const user = userEvent.setup()
    const trigger = await screen.findByRole('combobox', {
      name: 'Zona horaria',
    })
    expect(trigger).toHaveTextContent('America/Buenos_Aires')
    await user.click(trigger)
    expect(
      await screen.findByRole('option', { name: 'America/Buenos_Aires' }),
    ).toHaveAttribute('aria-selected', 'true')
  })

  it('sends only the new zone when it is the only change', async () => {
    const state = serveCompany()
    await renderCompany()
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('combobox', { name: 'Zona horaria' }),
    )
    await user.click(
      await screen.findByRole('option', { name: 'America/Argentina/Cordoba' }),
    )
    await user.click(save())
    await waitFor(() =>
      expect(state.patches).toEqual([
        { timezone: 'America/Argentina/Cordoba' },
      ]),
    )
    expect(state.tenant.updated_at).toBe(savedAt)
  })
})

describe('CompanyPage patch invariants', () => {
  beforeAll(preloadCompanyModules)
  it('never sends base_currency, nor null for name or timezone, whatever is edited', async () => {
    const state = serveCompany()
    await renderCompany()
    const user = userEvent.setup()
    for (const label of [
      'Razón social',
      'CUIT',
      'Dirección',
      'Teléfono',
      'Email de contacto',
    ]) {
      await user.clear(await field(label))
    }
    await user.click(save())
    await waitFor(() => expect(state.patches).toHaveLength(1))
    expect(state.patches[0]).toEqual({
      legal_name: null,
      tax_id: null,
      address: null,
      phone: null,
      email: null,
    })
    expectSafePatch(state.patches[0]!)
    expect(state.patches[0]).not.toHaveProperty('base_currency')
  })

  it('does nothing when the submit has no dirty field', async () => {
    const state = serveCompany()
    await renderCompany()
    await field('Teléfono')
    await act(async () => {
      screen
        .getByRole('button', { name: 'Guardar cambios' })
        .closest('form')
        ?.requestSubmit()
    })
    expect(state.patches).toHaveLength(0)
  })
})
