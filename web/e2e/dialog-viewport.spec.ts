import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'

// T-002-F202 (PR #79 review, P2): in a short viewport the shared dialog must fit it and scroll, so
// the title and "Cancelar" of "Nueva etapa" stay reachable with the pointer, touch and keyboard.
test('"Nueva etapa" fits a 375x812 viewport and its content scrolls', async ({
  page,
}) => {
  test.setTimeout(120_000)
  const name = 'Persona de prueba'
  await page.setViewportSize({ width: 375, height: 812 })
  await page.goto('/signup')
  await page.getByLabel('Tu nombre').fill(name)
  await page.getByLabel('Email').fill(`dialog-${randomUUID()}@example.test`)
  await page
    .getByLabel('Contraseña')
    .fill('Fictional dialog test password 123!')
  await page
    .getByLabel('Nombre de la empresa')
    .fill(`Empresa dialogo E2E ${randomUUID().slice(0, 8)}`)
  await page.getByRole('radio', { name: 'Genérico' }).check()
  await page.getByRole('button', { name: 'Crear cuenta' }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: `Hola, ${name}` }),
  ).toBeVisible()

  await page.goto('/settings/stages')
  await page.getByRole('button', { name: '+ Agregar etapa' }).click()
  const dialog = page.getByRole('dialog', { name: 'Nueva etapa' })
  await expect(dialog).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Nombre' })).toBeFocused()

  const viewport = page.viewportSize()!
  const box = (await dialog.boundingBox())!
  expect(box.y).toBeGreaterThanOrEqual(0)
  expect(box.y + box.height).toBeLessThanOrEqual(viewport.height)
  const metrics = await dialog.evaluate((element) => ({
    scrollHeight: element.scrollHeight,
    clientHeight: element.clientHeight,
    overflowY: getComputedStyle(element).overflowY,
  }))
  expect(metrics.overflowY).toBe('auto')
  expect(metrics.scrollHeight).toBeGreaterThan(metrics.clientHeight)

  // The content scrolls, so the actions at the bottom come into view and the keyboard reaches them.
  const cancel = dialog.getByRole('button', { name: 'Cancelar' })
  await cancel.scrollIntoViewIfNeeded()
  const cancelBox = (await cancel.boundingBox())!
  expect(cancelBox.y).toBeGreaterThanOrEqual(0)
  expect(cancelBox.y + cancelBox.height).toBeLessThanOrEqual(viewport.height)
  await cancel.focus()
  await page.keyboard.press('Enter')
  await expect(dialog).toBeHidden()

  // The title is back in reach after scrolling up.
  await page.getByRole('button', { name: '+ Agregar etapa' }).click()
  const title = dialog.getByRole('heading', { name: 'Nueva etapa' })
  await title.scrollIntoViewIfNeeded()
  const titleBox = (await title.boundingBox())!
  expect(titleBox.y).toBeGreaterThanOrEqual(0)
})
