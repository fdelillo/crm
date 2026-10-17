import type {
  CurrentUser,
  ErrorCode,
  IndustryTemplate,
  Problem,
  SessionInfo,
  Tenant,
  TenantSummary,
  ValidationProblem,
  User,
} from '@/api/types'

const userId = '0199bada-0000-7000-8000-000000000001'
const tenantId = '0199bada-0000-7000-8000-000000000002'
const timestamp = '2026-10-15T12:00:00Z'

export function buildCurrentUser(
  overrides: Partial<CurrentUser> = {},
): CurrentUser {
  return {
    id: userId,
    name: 'Persona de prueba',
    email: 'persona@example.test',
    role: 'admin',
    status: 'active',
    email_verified: false,
    ...overrides,
  }
}
export function buildTenant(overrides: Partial<Tenant> = {}): Tenant {
  return {
    id: tenantId,
    name: 'Empresa de prueba',
    legal_name: null,
    tax_id: null,
    address: null,
    phone: null,
    email: null,
    has_logo: false,
    base_currency: 'ARS',
    timezone: 'America/Argentina/Buenos_Aires',
    industry_template_code: 'generic',
    created_at: timestamp,
    updated_at: timestamp,
    ...overrides,
  }
}
export function buildIndustryTemplates(
  items: IndustryTemplate[] = [{ code: 'generic', name: 'Genérico' }],
): { items: IndustryTemplate[] } {
  return { items }
}
export function buildSessionInfo(
  overrides: Partial<SessionInfo> = {},
): SessionInfo {
  const { id, name, base_currency, timezone, has_logo } = buildTenant()
  const tenant: TenantSummary = { id, name, base_currency, timezone, has_logo }
  return {
    user: buildCurrentUser(),
    tenant,
    permissions: ['settings.manage'],
    ...overrides,
  }
}
export function buildUser(overrides: Partial<User> = {}): User {
  return {
    ...buildCurrentUser(),
    created_at: timestamp,
    invitation_expires_at: null,
    ...overrides,
  }
}
export function problem(
  code: ErrorCode,
  status: number,
  extra: Partial<ValidationProblem> = {},
): Problem {
  return {
    type: `/problems/${code}`,
    title: 'Error de prueba',
    status,
    code,
    ...extra,
  }
}
