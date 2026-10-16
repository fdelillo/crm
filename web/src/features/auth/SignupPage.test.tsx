import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { buildSessionInfo, problem } from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'

const base = 'https://localhost:5173/api/v1'
const templates = [
  { code: 'aluminum_carpentry', name: 'Carpintería de aluminio' },
  { code: 'generic', name: 'Genérico' },
]
function setupCatalog() {
  server.use(
    http.get(`${base}/me`, () =>
      HttpResponse.json(problem('unauthenticated', 401), {
        status: 401,
        headers: { 'Content-Type': 'application/problem+json' },
      }),
    ),
    http.get(`${base}/industry-templates`, () => HttpResponse.json({ items: templates })),
  )
}
async function completeForm(user = userEvent.setup()) {
  await user.type(screen.getByLabelText('Tu nombre'), ' Ana Pérez ')
  await user.type(screen.getByLabelText('Email'), ' ANA@example.test ')
  await user.type(screen.getByLabelText('Contraseña'), 'contraseña-segura')
  await user.type(screen.getByLabelText('Nombre de la empresa'), ' Aberturas Norte ')
  await user.click(screen.getByRole('radio', { name: 'Genérico' }))
  return user
}
function serverProblem(code: Parameters<typeof problem>[0], status: number, headers?: HeadersInit) {
  return HttpResponse.json(problem(code, status), {
    status,
    headers: { 'Content-Type': 'application/problem+json', ...Object.fromEntries(new Headers(headers)) },
  })
}

describe('SignupPage', () => {
  it('shows template loading placeholders and keeps submit disabled until loaded', async () => {
    setupCatalog()
    server.use(http.get(`${base}/industry-templates`, async () => new Promise(() => {})))
    renderApp(['/signup'])
    expect(await screen.findByRole('status', { name: 'Cargando rubros' })).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByRole('button', { name: 'Crear cuenta' })).toBeDisabled()
  })

  it('retries template loading after an error', async () => {
    setupCatalog()
    const list = vi.fn()
      .mockImplementationOnce(() => serverProblem('service_unavailable', 503))
      .mockImplementationOnce(() => HttpResponse.json({ items: templates }))
    server.use(http.get(`${base}/industry-templates`, list))
    renderApp(['/signup'])
    expect(await screen.findByText('No pudimos cargar los rubros.')).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(await screen.findByRole('radio', { name: 'Carpintería de aluminio' })).toBeVisible()
    expect(screen.getByRole('radio', { name: 'Genérico' })).toBeVisible()
  })

  it('validates required fields, focuses the first error and makes no request', async () => {
    setupCatalog()
    const signup = vi.fn(() => HttpResponse.json({}))
    server.use(http.post(`${base}/auth/signup`, signup))
    renderApp(['/signup'])
    await screen.findByRole('radio', { name: 'Genérico' })
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByText('Completá este campo.')).toBeVisible()
    expect(screen.getByText('Ingresá un email válido, por ejemplo nombre@empresa.com')).toBeVisible()
    expect(screen.getByText('La contraseña tiene que tener al menos 10 caracteres.')).toBeVisible()
    expect(screen.getByText('Elegí el rubro de tu empresa.')).toBeVisible()
    expect(screen.getByLabelText('Tu nombre')).toHaveFocus()
    expect(signup).not.toHaveBeenCalled()
  })

  it('posts normalized values and the browser timezone once, then replaces the form with the dashboard', async () => {
    setupCatalog()
    const body: unknown[] = []
    server.use(
      http.post(`${base}/auth/signup`, async ({ request }) => {
        body.push(await request.json())
        return HttpResponse.json(buildSessionInfo({
          user: { ...buildSessionInfo().user, name: 'Ana Pérez', email: 'ana@example.test' },
          tenant: { ...buildSessionInfo().tenant, name: 'Aberturas Norte' },
        }), { status: 201 })
      }),
      http.get(`${base}/tenant`, () => HttpResponse.json({
        id: 'tenant-id', name: 'Aberturas Norte', legal_name: null, tax_id: null,
        address: null, phone: null, email: null, has_logo: false, base_currency: 'ARS',
        timezone: 'America/Argentina/Buenos_Aires', industry_template_code: 'generic',
        created_at: '2026-10-16T12:00:00Z', updated_at: '2026-10-16T12:00:00Z',
      })),
    )
    renderApp(['/signup'])
    await completeForm()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByRole('heading', { name: 'Hola, Ana Pérez' })).toBeVisible()
    expect(await screen.findByText('¡Listo! Tu empresa quedó creada.')).toBeVisible()
    expect(body).toEqual([{
      name: 'Ana Pérez', email: 'ana@example.test', password: 'contraseña-segura',
      company_name: 'Aberturas Norte', base_currency: 'ARS',
      industry_template_code: 'generic',
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    }])
  })

  it('shows the existing-email recovery action and returns to signup with focus on email', async () => {
    setupCatalog()
    server.use(http.post(`${base}/auth/signup`, () => serverProblem('email_already_registered', 409)))
    const { router } = renderApp(['/signup'])
    await completeForm()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Ya existe un usuario con ese email.')
    await userEvent.click(screen.getByRole('button', { name: 'Usar otro email' }))
    expect(screen.getByLabelText('Email')).toHaveFocus()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    await userEvent.click(screen.getByRole('button', { name: 'Recuperar contraseña' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/forgot-password'))
    expect(router.state.location.state).toEqual({ email: 'ana@example.test' })
  })

  it('maps signup validation errors to their fields and focuses the field', async () => {
    setupCatalog()
    server.use(http.post(`${base}/auth/signup`, () => HttpResponse.json(
      problem('validation_failed', 422, { errors: [{ field: 'company_name', code: 'invalid_value' }] }),
      { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
    )))
    renderApp(['/signup'])
    await completeForm()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByText('Tiene que tener entre 1 y 120 caracteres.')).toBeVisible()
    expect(screen.getByLabelText('Nombre de la empresa')).toHaveFocus()
  })

  it.each([
    ['720', 'Esperá 12 minutos y probá de nuevo. No se creó nada: podés reintentar con los mismos datos.'],
    ['45', 'Esperá unos segundos y probá de nuevo. No se creó nada: podés reintentar con los mismos datos.'],
  ])('shows rate limit wait %s without clearing entered values or auto retrying', async (retryAfter, description) => {
    setupCatalog()
    const signup = vi.fn(() => serverProblem('rate_limited', 429, { 'Retry-After': retryAfter }))
    server.use(http.post(`${base}/auth/signup`, signup))
    renderApp(['/signup'])
    await completeForm()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(description)
    expect(screen.getByLabelText('Nombre de la empresa')).toHaveValue(' Aberturas Norte ')
    expect(signup).toHaveBeenCalledOnce()
  })

  it.each([
    ['503 without retry header', () => serverProblem('service_unavailable', 503)],
    ['network failure', () => HttpResponse.error()],
    ['503 with a short retry header', () => serverProblem('service_unavailable', 503, { 'Retry-After': '2' })],
  ])('preserves values on %s and retries only after the user asks', async (_name, response) => {
    setupCatalog()
    const signup = vi.fn().mockImplementationOnce(response).mockImplementationOnce(() => HttpResponse.json(buildSessionInfo(), { status: 201 }))
    server.use(http.post(`${base}/auth/signup`, signup))
    renderApp(['/signup'])
    await completeForm()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Reintentar')
    expect(signup).toHaveBeenCalledOnce()
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(await screen.findByRole('heading', { name: 'Hola, Ana Pérez' })).toBeVisible()
    expect(signup).toHaveBeenCalledTimes(2)
  })

  it('prevents a double submission and shows pending state', async () => {
    setupCatalog()
    let finish!: (response: Response) => void
    const signup = vi.fn(() => new Promise<Response>((resolve) => { finish = resolve }))
    server.use(http.post(`${base}/auth/signup`, signup))
    renderApp(['/signup'])
    const user = await completeForm()
    const button = screen.getByRole('button', { name: 'Crear cuenta' })
    await user.dblClick(button)
    expect(await screen.findByRole('button', { name: 'Creando cuenta…' })).toBeDisabled()
    expect(signup).toHaveBeenCalledOnce()
    finish(HttpResponse.json(buildSessionInfo(), { status: 201 }))
    await screen.findByRole('heading', { name: 'Hola, Ana Pérez' })
  })

  it('exposes labels and autocomplete, allows keyboard radio selection, and uses ARS by default', async () => {
    setupCatalog()
    renderApp(['/signup'])
    await screen.findByRole('radio', { name: 'Genérico' })
    for (const [label, autocomplete] of [
      ['Tu nombre', 'name'], ['Email', 'email'], ['Contraseña', 'new-password'], ['Nombre de la empresa', 'organization'],
    ]) {
      expect(screen.getByLabelText(label)).toHaveAttribute('autocomplete', autocomplete)
    }
    expect(screen.getByText('La usamos para totales y reportes. No se puede cambiar después.')).toBeVisible()
    expect(screen.getByRole('radio', { name: 'Pesos (ARS)' })).toBeChecked()
    const template = screen.getByRole('radio', { name: 'Carpintería de aluminio' })
    template.focus()
    await userEvent.keyboard('{ArrowDown}')
    expect(screen.getByRole('radio', { name: 'Genérico' })).toBeChecked()
  })

  it('redirects a visitor with a current session to the panel', async () => {
    server.use(
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
      http.get(`${base}/tenant`, () => HttpResponse.json({
        id: 'tenant-id', name: 'Empresa de prueba', legal_name: null, tax_id: null,
        address: null, phone: null, email: null, has_logo: false, base_currency: 'ARS',
        timezone: 'America/Argentina/Buenos_Aires', industry_template_code: 'generic',
        created_at: '2026-10-16T12:00:00Z', updated_at: '2026-10-16T12:00:00Z',
      })),
      http.get(`${base}/industry-templates`, () => HttpResponse.json({ items: templates })),
    )
    const { router } = renderApp(['/signup'])
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  })
})
