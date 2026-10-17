import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import {
  buildIndustryTemplates,
  buildSessionInfo,
  buildTenant,
  problem,
} from '@/test/fixtures'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'

const base = 'https://localhost:5173/api/v1'
const templates = buildIndustryTemplates([
  { code: 'aluminum_carpentry', name: 'Carpintería de aluminio' },
  { code: 'generic', name: 'Genérico' },
])
const tenantResponse = () =>
  HttpResponse.json(buildTenant({ name: 'Aberturas Norte' }))
function setupCatalog() {
  server.use(
    http.get(`${base}/me`, () =>
      HttpResponse.json(problem('unauthenticated', 401), {
        status: 401,
        headers: { 'Content-Type': 'application/problem+json' },
      }),
    ),
    http.get(`${base}/industry-templates`, () => HttpResponse.json(templates)),
  )
}
async function completeForm(user = userEvent.setup()) {
  await user.type(await screen.findByLabelText('Tu nombre'), ' Ana Pérez ')
  await user.type(screen.getByLabelText('Email'), ' ANA@example.test ')
  await user.type(screen.getByLabelText('Contraseña'), 'contraseña-segura')
  await user.type(
    screen.getByLabelText('Nombre de la empresa'),
    ' Aberturas Norte ',
  )
  await user.click(screen.getByRole('radio', { name: 'Genérico' }))
  return user
}
function serverProblem(
  code: Parameters<typeof problem>[0],
  status: number,
  headers?: Record<string, string>,
) {
  return HttpResponse.json(problem(code, status), {
    status,
    headers: { 'Content-Type': 'application/problem+json', ...headers },
  })
}

describe('SignupPage', () => {
  // T-F202: registration form validation, keyboard use, and API error handling.
  it('shows template loading placeholders and keeps submit disabled until loaded', async () => {
    setupCatalog()
    server.use(
      http.get(`${base}/industry-templates`, async () => new Promise(() => {})),
    )
    renderApp(['/signup'])
    expect(
      await screen.findByRole('status', { name: 'Cargando rubros' }),
    ).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByRole('button', { name: 'Crear cuenta' })).toBeDisabled()
  })

  it('retries template loading after an error', async () => {
    setupCatalog()
    let recovered = false
    const list = vi.fn(() =>
      recovered
        ? HttpResponse.json(templates)
        : serverProblem('service_unavailable', 503),
    )
    server.use(http.get(`${base}/industry-templates`, list))
    renderApp(['/signup'])
    expect(
      await screen.findByText('No pudimos cargar los rubros.'),
    ).toBeVisible()
    recovered = true
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(
      await screen.findByRole('radio', { name: 'Carpintería de aluminio' }),
    ).toBeVisible()
    expect(screen.getByRole('radio', { name: 'Genérico' })).toBeVisible()
  })

  it('validates required fields, focuses the first error and makes no request', async () => {
    setupCatalog()
    const signup = vi.fn(() => HttpResponse.json({}))
    server.use(http.post(`${base}/auth/signup`, signup))
    renderApp(['/signup'])
    await screen.findByRole('radio', { name: 'Genérico' })
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect((await screen.findAllByText('Completá este campo.')).length).toBe(3)
    expect(
      screen.getByText('La contraseña tiene que tener al menos 10 caracteres.'),
    ).toBeVisible()
    expect(screen.getByText('Elegí el rubro de tu empresa.')).toBeVisible()
    expect(screen.getByLabelText('Tu nombre')).toHaveFocus()
    expect(signup).not.toHaveBeenCalled()
  })

  it('posts normalized values and the browser timezone once, then replaces the form with the dashboard', async () => {
    setupCatalog()
    const browserOptions = new Intl.DateTimeFormat().resolvedOptions()
    const timezoneSpy = vi
      .spyOn(Intl.DateTimeFormat.prototype, 'resolvedOptions')
      .mockReturnValue({ ...browserOptions, timeZone: 'Etc/Unknown' })
    const body: unknown[] = []
    let registered = false
    const session = buildSessionInfo({
      user: {
        ...buildSessionInfo().user,
        name: 'Ana Pérez',
        email: 'ana@example.test',
      },
      tenant: { ...buildSessionInfo().tenant, name: 'Aberturas Norte' },
    })
    server.use(
      http.get(`${base}/me`, () =>
        registered
          ? HttpResponse.json(session)
          : serverProblem('unauthenticated', 401),
      ),
      http.post(`${base}/auth/signup`, async ({ request }) => {
        body.push(await request.json())
        registered = true
        return HttpResponse.json(session, { status: 201 })
      }),
      http.get(`${base}/tenant`, tenantResponse),
    )
    const { router } = renderApp(['/login', '/signup'])
    await completeForm()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(router.state.historyAction).toBe('REPLACE')
    await router.navigate(-1)
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(
      (await screen.findAllByText('¡Listo! Tu empresa quedó creada.')).length,
    ).toBeGreaterThan(0)
    expect(body).toEqual([
      {
        name: 'Ana Pérez',
        email: 'ana@example.test',
        password: 'contraseña-segura',
        company_name: 'Aberturas Norte',
        base_currency: 'ARS',
        industry_template_code: 'generic',
        timezone: 'Etc/Unknown',
      },
    ])
    timezoneSpy.mockRestore()
  })

  it('shows the existing-email recovery action and returns to signup with focus on email', async () => {
    setupCatalog()
    server.use(
      http.post(`${base}/auth/signup`, () =>
        HttpResponse.json(
          problem('email_already_registered', 409, {
            suggested_action: 'password_reset',
          }),
          {
            status: 409,
            headers: { 'Content-Type': 'application/problem+json' },
          },
        ),
      ),
    )
    const { router } = renderApp(['/signup'])
    await completeForm()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Ya existe un usuario con ese email.',
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Usar otro email' }),
    )
    expect(screen.getByLabelText('Email')).toHaveFocus()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    await userEvent.click(
      screen.getByRole('button', { name: 'Recuperar contraseña' }),
    )
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/forgot-password'),
    )
    expect(router.state.location.state).toEqual({ email: 'ana@example.test' })
  })

  it('maps signup validation errors to their fields and focuses the field', async () => {
    setupCatalog()
    server.use(
      http.post(`${base}/auth/signup`, () =>
        HttpResponse.json(
          problem('validation_failed', 422, {
            errors: [{ field: 'company_name', code: 'invalid_value' }],
          }),
          {
            status: 422,
            headers: { 'Content-Type': 'application/problem+json' },
          },
        ),
      ),
    )
    renderApp(['/signup'])
    await completeForm()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(
      await screen.findByText('Tiene que tener entre 1 y 120 caracteres.'),
    ).toBeVisible()
    expect(screen.getByLabelText('Nombre de la empresa')).toHaveFocus()
  })

  it('focuses the first industry radio when every other field is valid', async () => {
    setupCatalog()
    renderApp(['/signup'])
    await screen.findByRole('radio', { name: 'Carpintería de aluminio' })
    await userEvent.type(screen.getByLabelText('Tu nombre'), 'Ana Pérez')
    await userEvent.type(screen.getByLabelText('Email'), 'ana@example.test')
    await userEvent.type(
      screen.getByLabelText('Contraseña'),
      'contraseña-segura',
    )
    await userEvent.type(
      screen.getByLabelText('Nombre de la empresa'),
      'Aberturas Norte',
    )
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(
      screen.getByRole('radio', { name: 'Carpintería de aluminio' }),
    ).toHaveFocus()
  })

  it('focuses the industry radio after an unknown-template server error', async () => {
    setupCatalog()
    let catalogLoads = 0
    server.use(
      http.get(`${base}/industry-templates`, () => {
        catalogLoads += 1
        return HttpResponse.json(templates)
      }),
      http.post(`${base}/auth/signup`, () =>
        HttpResponse.json(
          problem('validation_failed', 422, {
            errors: [
              { field: 'industry_template_code', code: 'unknown_template' },
            ],
          }),
          {
            status: 422,
            headers: { 'Content-Type': 'application/problem+json' },
          },
        ),
      ),
    )
    renderApp(['/signup'])
    await completeForm()
    await userEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByText('Elegí un rubro de la lista.')).toBeVisible()
    expect(screen.getByRole('radio', { name: 'Genérico' })).toHaveFocus()
    await waitFor(() => expect(catalogLoads).toBeGreaterThan(1))
  })

  it.each([
    [
      '720',
      'Esperá 12 minutos y probá de nuevo. No se creó nada: podés reintentar con los mismos datos.',
    ],
    [
      '45',
      'Esperá unos segundos y probá de nuevo. No se creó nada: podés reintentar con los mismos datos.',
    ],
  ])(
    'shows rate limit wait %s without clearing entered values or auto retrying',
    async (retryAfter, description) => {
      setupCatalog()
      const signup = vi.fn(() =>
        serverProblem('rate_limited', 429, { 'Retry-After': retryAfter }),
      )
      server.use(http.post(`${base}/auth/signup`, signup))
      renderApp(['/signup'])
      await completeForm()
      await userEvent.click(
        screen.getByRole('button', { name: 'Crear cuenta' }),
      )
      expect(
        await screen.findByText('Hubo muchos intentos de registro seguidos.'),
      ).toBeVisible()
      expect(screen.getByRole('alert')).toHaveTextContent(description)
      expect(screen.getByLabelText('Nombre de la empresa')).toHaveValue(
        ' Aberturas Norte ',
      )
      expect(signup).toHaveBeenCalledOnce()
      await userEvent.click(
        screen.getByRole('button', { name: 'Crear cuenta' }),
      )
      await waitFor(() => expect(signup).toHaveBeenCalledTimes(2))
    },
  )

  it.each([
    [
      '503 without retry header',
      () => serverProblem('service_unavailable', 503),
    ],
    ['network failure', () => HttpResponse.error()],
    [
      '503 with a short retry header',
      () => serverProblem('service_unavailable', 503, { 'Retry-After': '2' }),
    ],
  ])(
    'preserves values on %s and retries only after the user asks',
    async (name, response) => {
      setupCatalog()
      let registered = false
      const session = buildSessionInfo({
        user: { ...buildSessionInfo().user, name: 'Ana Pérez' },
      })
      const signup = vi
        .fn()
        .mockImplementationOnce(response)
        .mockImplementationOnce(() => {
          registered = true
          return HttpResponse.json(session, { status: 201 })
        })
      server.use(
        http.get(`${base}/me`, () =>
          registered
            ? HttpResponse.json(session)
            : serverProblem('unauthenticated', 401),
        ),
        http.get(`${base}/tenant`, tenantResponse),
        http.post(`${base}/auth/signup`, signup),
      )
      const { router } = renderApp(['/signup'])
      await completeForm()
      await userEvent.click(
        screen.getByRole('button', { name: 'Crear cuenta' }),
      )
      const alert = await screen.findByRole('alert')
      if (name === '503 with a short retry header') {
        expect(alert).toHaveTextContent(
          'Probá de nuevo en unos segundos. No se creó nada: podés reintentar con los mismos datos.',
        )
      } else if (name === 'network failure') {
        expect(alert).toHaveTextContent('No hay conexión.')
      }
      expect(screen.getByLabelText('Tu nombre')).toHaveValue(' Ana Pérez ')
      expect(screen.getByLabelText('Email')).toHaveValue('ANA@example.test')
      expect(screen.getByLabelText('Contraseña')).toHaveValue(
        'contraseña-segura',
      )
      expect(signup).toHaveBeenCalledOnce()
      await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
      await waitFor(() => expect(router.state.location.pathname).toBe('/'))
      expect(
        (await screen.findAllByText('¡Listo! Tu empresa quedó creada.')).length,
      ).toBeGreaterThan(0)
      expect(signup).toHaveBeenCalledTimes(2)
    },
  )

  it('prevents a double submission and shows pending state', async () => {
    setupCatalog()
    let finish!: (response: Response) => void
    let registered = false
    const session = buildSessionInfo({
      user: { ...buildSessionInfo().user, name: 'Ana Pérez' },
    })
    const signup = vi.fn(
      () =>
        new Promise<Response>((resolve) => {
          finish = (response) => {
            registered = true
            resolve(response)
          }
        }),
    )
    server.use(
      http.get(`${base}/me`, () =>
        registered
          ? HttpResponse.json(session)
          : serverProblem('unauthenticated', 401),
      ),
      http.get(`${base}/tenant`, tenantResponse),
      http.post(`${base}/auth/signup`, signup),
    )
    const { router } = renderApp(['/signup'])
    const user = await completeForm()
    const button = screen.getByRole('button', { name: 'Crear cuenta' })
    await user.dblClick(button)
    expect(
      await screen.findByRole('button', { name: 'Creando cuenta…' }),
    ).toBeDisabled()
    expect(signup).toHaveBeenCalledOnce()
    expect(screen.getByRole('radio', { name: 'Genérico' })).toBeDisabled()
    expect(screen.getByRole('radio', { name: 'Dólares (USD)' })).toBeDisabled()
    finish(HttpResponse.json(buildSessionInfo(), { status: 201 }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  })

  it('completes and submits signup with the keyboard', async () => {
    setupCatalog()
    let registered = false
    const session = buildSessionInfo({
      user: { ...buildSessionInfo().user, name: 'Ana Pérez' },
    })
    const signup = vi.fn(() => {
      registered = true
      return HttpResponse.json(session, { status: 201 })
    })
    server.use(
      http.get(`${base}/me`, () =>
        registered
          ? HttpResponse.json(session)
          : serverProblem('unauthenticated', 401),
      ),
      http.get(`${base}/tenant`, tenantResponse),
      http.post(`${base}/auth/signup`, signup),
    )
    const { router } = renderApp(['/signup'])
    await screen.findByRole('radio', { name: 'Genérico' })
    const user = userEvent.setup()
    const name = screen.getByLabelText('Tu nombre')
    for (let tab = 0; document.activeElement !== name && tab < 10; tab += 1) {
      await user.tab()
    }
    expect(name).toHaveFocus()

    await user.type(name, 'Ana Pérez')
    await user.tab()
    const email = screen.getByLabelText('Email')
    expect(email).toHaveFocus()
    await user.type(email, 'ana@example.test')
    await user.tab()
    const password = screen.getByLabelText('Contraseña')
    expect(password).toHaveFocus()
    await user.type(password, 'contraseña-segura')
    await user.tab()
    await user.tab()
    const company = screen.getByLabelText('Nombre de la empresa')
    expect(company).toHaveFocus()
    await user.type(company, 'Aberturas Norte')
    await user.tab()
    expect(
      screen.getByRole('radio', { name: 'Carpintería de aluminio' }),
    ).toHaveFocus()
    await user.keyboard(' ')
    expect(
      screen.getByRole('radio', { name: 'Carpintería de aluminio' }),
    ).toBeChecked()
    await user.tab()
    await user.tab()
    expect(screen.getByRole('button', { name: 'Crear cuenta' })).toHaveFocus()
    await user.keyboard('{Enter}')

    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(signup).toHaveBeenCalledOnce()
  })

  it('exposes labels and autocomplete, allows keyboard radio selection, and uses ARS by default', async () => {
    setupCatalog()
    renderApp(['/signup'])
    await screen.findByRole('radio', { name: 'Genérico' })
    for (const [label, autocomplete] of [
      ['Tu nombre', 'name'],
      ['Email', 'email'],
      ['Contraseña', 'new-password'],
      ['Nombre de la empresa', 'organization'],
    ] as const) {
      expect(screen.getByLabelText(label)).toHaveAttribute(
        'autocomplete',
        autocomplete,
      )
    }
    expect(
      screen.getByText(
        'La usamos para totales y reportes. No se puede cambiar después.',
      ),
    ).toBeVisible()
    expect(screen.getByRole('radio', { name: 'Pesos (ARS)' })).toBeChecked()
    const user = userEvent.setup()
    const template = screen.getByRole('radio', {
      name: 'Carpintería de aluminio',
    })
    template.focus()
    await user.keyboard(' ')
    expect(
      screen.getByRole('radio', { name: 'Carpintería de aluminio' }),
    ).toBeChecked()
  })

  it('redirects a visitor with a current session to the panel', async () => {
    server.use(
      http.get(`${base}/me`, () => HttpResponse.json(buildSessionInfo())),
      http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
      http.get(`${base}/industry-templates`, () =>
        HttpResponse.json(templates),
      ),
    )
    const { router } = renderApp(['/signup'])
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  })
})
