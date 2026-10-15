import { expect, test } from '@playwright/test'

// T-F009: the embedded build exposes the provisional login screen over trusted HTTPS.
test('opens the embedded application', async ({ page }) => {
  await page.goto('/login')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Ingresar', exact: true }),
  ).toBeVisible()
})
