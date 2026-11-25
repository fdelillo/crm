import { expect, type Locator, type Page } from '@playwright/test'

// T-002-F103 (002/DD-F13, DD-F12, P-F2, P-F3): geometry checks for the hallazgos 1, 2, 3 and 5 of
// T-F705 de 001. They measure what is rendered, never CSS classes.

async function box(locator: Locator) {
  const bounds = await locator.boundingBox()
  expect(bounds).not.toBeNull()
  return bounds!
}

// The target of a radio is the card <label> that wraps it, not the 16 px circle.
function radioCard(page: Page, name: string) {
  return page
    .locator('label')
    .filter({ has: page.getByRole('radio', { name }) })
}

// Hallazgo 2: the radio keeps its circle and its card is the 44 px target.
export async function expectCircleRadios(page: Page, names: string[]) {
  for (const name of names) {
    expect(
      (await box(page.getByRole('radio', { name }))).height,
    ).toBeLessThanOrEqual(24)
    expect((await box(radioCard(page, name))).height).toBeGreaterThanOrEqual(44)
  }
}

// Signup has a per-IP quota (5) that each CI group already spends, so this runs inside the
// journey of a verified administrator instead of registering a company of its own. It returns
// the page to /settings/users at the width it had.
export async function expectConfigurationTouchTargets(
  page: Page,
  administratorName: string,
) {
  const original = page.viewportSize()
  await page.setViewportSize({ width: 375, height: 812 })

  // P-F3 (hallazgo 5): neighbouring links of "Tu empresa" are 44 px tall and 8 px apart.
  await page.goto('/settings')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Ajustes' }),
  ).toBeVisible()
  const company = await box(
    page.getByRole('link', { name: 'Datos de la empresa' }),
  )
  const users = await box(
    page.getByRole('link', { name: 'Usuarios', exact: true }),
  )
  expect(company.height).toBeGreaterThanOrEqual(44)
  expect(users.height).toBeGreaterThanOrEqual(44)
  expect(users.y - (company.y + company.height)).toBeGreaterThanOrEqual(8)

  // P-F2 (hallazgos 1 y 2): links that look like buttons are 44 px; role radios stay circles.
  await page.goto('/settings/users')
  const invite = page.getByRole('link', { name: '+ Invitar' })
  await expect(invite).toBeVisible()
  expect((await box(invite)).height).toBeGreaterThanOrEqual(44)
  await invite.click()
  await expect(
    page.getByRole('heading', { level: 1, name: 'Invitar a alguien' }),
  ).toBeVisible()
  await expectCircleRadios(page, ['Administrador', 'Operador'])
  expect(
    (await box(page.getByRole('link', { name: 'Cancelar' }))).height,
  ).toBeGreaterThanOrEqual(44)

  // P-F2 (hallazgo 3): a menu with a disabled action fits in the viewport and its text is not cut.
  for (const width of [375, 320]) {
    await page.setViewportSize({ width, height: 720 })
    await page.goto('/settings/users')
    await page
      .getByRole('button', { name: `Acciones para ${administratorName}` })
      .click()
    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()
    await expect(
      menu
        .getByText('Tiene que quedar al menos un administrador activo')
        .first(),
    ).toBeVisible()
    const bounds = await box(menu)
    expect(bounds.x).toBeGreaterThanOrEqual(0)
    expect(bounds.x + bounds.width).toBeLessThanOrEqual(width)
    expect(
      await menu.evaluate((element) => element.scrollWidth),
    ).toBeLessThanOrEqual(await menu.evaluate((element) => element.clientWidth))
    await page.keyboard.press('Escape')
  }

  if (original) await page.setViewportSize(original)
}
