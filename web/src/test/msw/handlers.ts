import type { RequestHandler } from 'msw'
import { http, HttpResponse } from 'msw'
import { buildIndustryTemplates } from '@/test/fixtures'

export const handlers: RequestHandler[] = [
  http.get('*/api/v1/industry-templates', () =>
    HttpResponse.json(buildIndustryTemplates()),
  ),
  http.get('*/api/v1/users', () => HttpResponse.json({ items: [] })),
  http.post('*/api/v1/auth/invitations/preview', () =>
    HttpResponse.json({
      tenant_name: 'Empresa de prueba',
      email: 'persona@example.test',
      role: 'operator',
      expires_at: '2026-10-27T12:00:00Z',
    }),
  ),
]
