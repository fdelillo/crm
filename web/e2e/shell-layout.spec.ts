import { expect, test } from '@playwright/test'
import type { SignupRequest } from '../src/api/types'

// T-F106/T-F107 (review: long valid company name): check rendered geometry, not CSS classes.
test('reserves the full company name above desktop navigation', async ({
  page,
}) => {
  const companyName = 'W'.repeat(120)
  const signup: SignupRequest = {
    name: 'Persona de prueba',
    email: `shell-layout-${Date.now()}@example.test`,
    password: 'Fictional test password 123!',
    company_name: companyName,
    base_currency: 'ARS',
    industry_template_code: 'generic',
    timezone: 'America/Argentina/Buenos_Aires',
  }
  // Setup uses the real API and its HttpOnly cookie; no application API is intercepted.
  const response = await page.request.post('/api/v1/auth/signup', {
    data: signup,
    headers: {
      Origin: new URL(
        test.info().project.use.baseURL ?? 'https://localhost:8443',
      ).origin,
    },
  })
  expect(response.status()).toBe(201)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/settings')
  await expect(
    page.getByRole('heading', { name: 'Ajustes', exact: true }),
  ).toBeVisible()
  const name = page.getByText(companyName, { exact: true })
  const navigation = page.getByRole('navigation', {
    name: 'Navegación principal',
  })
  try {
    for (const viewport of [
      { width: 1440, height: 900 },
      { width: 375, height: 812 },
      { width: 320, height: 640 },
    ]) {
      await page.setViewportSize(viewport)
      await expect(name).toBeVisible()
      await expect(
        navigation.getByRole('link', { name: 'Inicio', exact: true }),
      ).toBeVisible()
      await expect(
        navigation.getByRole('link', { name: 'Ajustes', exact: true }),
      ).toBeVisible()
      const brand = await name.boundingBox()
      const nav = await navigation.boundingBox()
      expect(brand).not.toBeNull()
      expect(nav).not.toBeNull()
      expect(brand!.y + brand!.height).toBeLessThanOrEqual(nav!.y)
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth),
      ).toBeLessThanOrEqual(viewport.width)
    }
  } finally {
    await page.request.post('/api/v1/auth/logout', {
      headers: {
        Origin: new URL(
          test.info().project.use.baseURL ?? 'https://localhost:8443',
        ).origin,
      },
    })
  }
})
