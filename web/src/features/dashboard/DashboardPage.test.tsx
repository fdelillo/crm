import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
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
const tenantResponse = (industryTemplateCode = 'aluminum_carpentry') =>
  HttpResponse.json(
    buildTenant({
      name: 'Aberturas Norte',
      base_currency: 'ARS',
      industry_template_code: industryTemplateCode,
    }),
  )
function installSession(role: 'admin' | 'operator') {
  // email_verified=false belongs to F5 (T-F504), not this F2 matrix.
  server.use(
    http.get(`${base}/me`, () =>
      HttpResponse.json(
        buildSessionInfo({
          user: {
            ...buildSessionInfo().user,
            name: 'Ana Pérez',
            role,
            email_verified: true,
          },
          permissions: role === 'admin' ? ['settings.manage'] : [],
        }),
      ),
    ),
    http.get(`${base}/industry-templates`, () => HttpResponse.json(templates)),
  )
}

describe('DashboardPage', () => {
  // T-F204: dashboard content follows the session permissions and loads independently.
  it('shows the admin greeting, company summary, and first steps', async () => {
    installSession('admin')
    server.use(http.get(`${base}/tenant`, () => tenantResponse()))
    renderApp(['/'])

    expect(
      await screen.findByRole('heading', { name: 'Hola, Ana Pérez' }),
    ).toBeVisible()
    const companyCard = await screen.findByText(
      'Rubro: Carpintería de aluminio',
    )
    expect(companyCard).toBeVisible()
    const card = companyCard.closest('[data-slot="card"]')
    expect(card).not.toBeNull()
    expect(
      within(card as HTMLElement).getByText('Aberturas Norte'),
    ).toBeVisible()
    expect(
      within(card as HTMLElement).getByText('Moneda base: Pesos (ARS)'),
    ).toBeVisible()
    expect(
      screen.getByRole('heading', { name: 'Primeros pasos' }),
    ).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Completá los datos de tu empresa' }),
    ).toHaveAttribute('href', '/settings/company')
    expect(
      screen.getByRole('link', { name: 'Invitá a tu equipo' }),
    ).toHaveAttribute('href', '/settings/users/invite')
  })

  it('hides first steps from an operator', async () => {
    installSession('operator')
    server.use(http.get(`${base}/tenant`, () => tenantResponse()))
    renderApp(['/'])

    expect(
      await screen.findByText('Rubro: Carpintería de aluminio'),
    ).toBeVisible()
    expect(screen.queryByText('Primeros pasos')).not.toBeInTheDocument()
  })

  it('keeps the greeting visible while the company card loads as a skeleton', async () => {
    installSession('admin')
    server.use(http.get(`${base}/tenant`, async () => new Promise(() => {})))
    renderApp(['/'])

    expect(
      await screen.findByRole('heading', { name: 'Hola, Ana Pérez' }),
    ).toBeVisible()
    expect(
      screen.getByRole('status', { name: 'Cargando datos de tu empresa' }),
    ).toHaveAttribute('aria-busy', 'true')
    expect(
      screen.getByRole('heading', { name: 'Primeros pasos' }),
    ).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Completá los datos de tu empresa' }),
    ).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Invitá a tu equipo' }),
    ).toBeVisible()
  })

  it('shows a retry action if company data fails to load', async () => {
    installSession('admin')
    server.use(
      http.get(`${base}/tenant`, () =>
        HttpResponse.json(problem('service_unavailable', 503), {
          status: 503,
          headers: { 'Content-Type': 'application/problem+json' },
        }),
      ),
    )
    renderApp(['/'])

    expect(
      await screen.findByText('No pudimos cargar los datos de tu empresa.'),
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeVisible()
    expect(
      screen.getByRole('heading', { name: 'Primeros pasos' }),
    ).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Completá los datos de tu empresa' }),
    ).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Invitá a tu equipo' }),
    ).toBeVisible()
  })

  it('shows an industry code that is missing from the current catalogue', async () => {
    installSession('admin')
    server.use(
      http.get(`${base}/tenant`, () => tenantResponse('custom_industry')),
    )
    renderApp(['/'])

    await waitFor(() =>
      expect(screen.getByText('Rubro: custom_industry')).toBeVisible(),
    )
  })
})
