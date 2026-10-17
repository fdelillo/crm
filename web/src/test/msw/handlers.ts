import type { RequestHandler } from 'msw'
import { http, HttpResponse } from 'msw'

export const handlers: RequestHandler[] = [
  http.get('*/api/v1/industry-templates', () =>
    HttpResponse.json({ items: [{ code: 'generic', name: 'Genérico' }] }),
  ),
]
