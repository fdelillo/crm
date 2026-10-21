import { http, HttpResponse } from 'msw'
import {
  buildCurrentUser,
  buildSessionInfo,
  buildTenant,
  problem,
} from '@/test/fixtures'
import { server } from '@/test/msw/server'
import type { Tenant, TenantUpdate } from '@/api/types'

export const base = 'https://localhost:5173/api/v1'
export const savedAt = '2026-10-16T09:00:00Z'

export function problemResponse(
  code: Parameters<typeof problem>[0],
  status: number,
  extra: Parameters<typeof problem>[2] = {},
  headers: Record<string, string> = {},
) {
  return HttpResponse.json(problem(code, status, extra), {
    status,
    headers: { 'Content-Type': 'application/problem+json', ...headers },
  })
}

export function companyTenant(overrides: Partial<Tenant> = {}) {
  return buildTenant({
    legal_name: 'ACME S.A.',
    tax_id: '30123456781',
    address: 'Calle Falsa 123',
    phone: '11 5555-0000',
    email: 'contacto@acme.test',
    ...overrides,
  })
}

/** An admin session with a confirmed email, a tenant that PATCH updates, and the PATCH bodies seen. */
export function serveCompany(initial: Tenant = companyTenant()) {
  const state = { tenant: initial, patches: [] as TenantUpdate[] }
  server.use(
    http.get(`${base}/me`, () =>
      HttpResponse.json(
        buildSessionInfo({
          user: buildCurrentUser({ email_verified: true }),
        }),
      ),
    ),
    http.get(`${base}/tenant`, () => HttpResponse.json(state.tenant)),
    http.patch(`${base}/tenant`, async ({ request }) => {
      const body = (await request.json()) as TenantUpdate
      state.patches.push(body)
      state.tenant = { ...state.tenant, ...body, updated_at: savedAt } as Tenant
      return HttpResponse.json(state.tenant)
    }),
  )
  return state
}

/** INV-F15: only changed TenantUpdate keys, never `base_currency`, never null name/timezone. */
export function expectSafePatch(body: Record<string, unknown>) {
  const allowed = [
    'name',
    'legal_name',
    'tax_id',
    'address',
    'phone',
    'email',
    'timezone',
  ]
  for (const [key, value] of Object.entries(body)) {
    if (!allowed.includes(key)) throw new Error(`Unexpected key ${key}`)
    if ((key === 'name' || key === 'timezone') && value === null)
      throw new Error(`${key} must never be null`)
  }
}

export async function preloadCompanyModules() {
  await Promise.all([
    import('@/features/auth/pages'),
    import('@/features/tenant/CompanyPage'),
    import('@/app/SettingsPage'),
    import('@/app/AppShell'),
  ])
}
