import type { RequestHandler } from 'msw'
import { http, HttpResponse } from 'msw'
import { buildIndustryTemplates } from '@/test/fixtures'

export const handlers: RequestHandler[] = [
  http.get('*/api/v1/industry-templates', () =>
    HttpResponse.json(buildIndustryTemplates()),
  ),
]
