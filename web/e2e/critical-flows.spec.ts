import { randomUUID } from 'node:crypto'
import { expect, test, type Page } from '@playwright/test'
import {
  captureBrowserDiagnostics,
  expectNoAccessibilityViolations,
  extractEmailLink,
  waitForEmail,
  type BrowserDiagnostics,
} from './support'

const origin = 'https://localhost:8443'
const initialPassword = 'Example test password 123!'
const resetPassword = 'Another example password 456!'

// T-F702: exercise the approved critical journeys through the real HTTPS binary and API.
test('critical registration, session, recovery, invitation, and access flows', async ({
  page,
  browser,
}) => {
  test.setTimeout(240_000)
  const diagnostics: BrowserDiagnostics[] = []
  const trackedPages: Page[] = []
  const track = async (target: Page) => {
    trackedPages.push(target)
    diagnostics.push(await captureBrowserDiagnostics(target))
  }
  const inspectScreen = async (target: Page) => {
    await expectNoAccessibilityViolations(target)
    expect(
      await target.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(320)
  }

  await page.setViewportSize({ width: 320, height: 720 })
  await track(page)

  const shellResponse = await page.goto('/')
  expect(shellResponse?.headers()['content-security-policy']).toBeTruthy()
  expect(shellResponse?.headers()['strict-transport-security']).toBeUndefined()
  await expect(
    page.getByRole('heading', { level: 1, name: 'Ingresá a tu cuenta' }),
  ).toBeVisible()
  await inspectScreen(page)

  const adminEmail = `f702-admin-${randomUUID()}@example.test`
  const adminName = 'Administradora de prueba'
  const companyName = `Empresa E2E ${randomUUID().slice(0, 8)}`
  await page.goto('/signup')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Creá tu cuenta' }),
  ).toBeVisible()
  await inspectScreen(page)
  await page.getByLabel('Tu nombre').fill(adminName)
  await page.getByLabel('Email').fill(adminEmail)
  await page.getByLabel('Contraseña').fill(initialPassword)
  await page.getByLabel('Nombre de la empresa').fill(companyName)
  await page.getByRole('radio', { name: 'Genérico' }).check()

  const signupStarted = Date.now()
  await page.getByRole('button', { name: 'Crear cuenta' }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: `Hola, ${adminName}` }),
  ).toBeVisible()
  expect(Date.now() - signupStarted).toBeLessThan(10_000)
  await expect(
    page.getByText(companyName, { exact: true }).first(),
  ).toBeVisible()
  await expect(page.getByText('Rubro: Genérico', { exact: true })).toBeVisible()
  await expect(
    page.getByText('Moneda base: Pesos (ARS)', { exact: true }),
  ).toBeVisible()
  await expect(
    page.getByText(
      `Confirmá tu email: te mandamos un enlace a ${adminEmail}.`,
      { exact: true },
    ),
  ).toBeVisible()

  const sessionCookie = (await page.context().cookies()).find(
    (cookie) => cookie.name === '__Host-crm_session',
  )
  expect(sessionCookie).toMatchObject({
    domain: 'localhost',
    httpOnly: true,
    path: '/',
    sameSite: 'Lax',
    secure: true,
  })
  await inspectScreen(page)

  await page
    .getByRole('navigation', { name: 'Navegación principal' })
    .getByRole('link', { name: 'Ajustes', exact: true })
    .click()
  await expect(
    page.getByRole('heading', { level: 1, name: 'Ajustes' }),
  ).toBeVisible()
  await inspectScreen(page)
  await page.getByRole('link', { name: 'Usuarios', exact: true }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: 'Usuarios' }),
  ).toBeVisible()
  await expect(
    page.getByText('Confirmá tu email para invitar a tu equipo.'),
  ).toBeVisible()
  await expect(page.getByRole('link', { name: '+ Invitar' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '+ Invitar' })).toBeDisabled()
  await inspectScreen(page)

  const verification = await waitForEmail('Verificá tu email', adminEmail)
  const verificationURL = extractEmailLink(verification)
  expect(verificationURL.origin).toBe(origin)
  const verificationPage = await page.context().newPage()
  await verificationPage.bringToFront()
  await verificationPage.setViewportSize({ width: 320, height: 720 })
  await track(verificationPage)
  await verificationPage.goto(verificationURL.toString())
  await expect(
    verificationPage.getByRole('button', { name: 'Confirmar mi email' }),
  ).toBeVisible()
  await inspectScreen(verificationPage)
  await verificationPage
    .getByRole('button', { name: 'Confirmar mi email' })
    .click()
  await expect(
    verificationPage.getByText('¡Listo! Tu email quedó confirmado.'),
  ).toBeVisible()
  await inspectScreen(verificationPage)
  const refreshedSession = page.waitForResponse((response) => {
    return (
      response.request().method() === 'GET' &&
      new URL(response.url()).pathname === '/api/v1/me'
    )
  })
  await page.bringToFront()
  // Headless Chromium keeps both pages visible; emit the lifecycle event that
  // a foreground-tab transition sends to React Query's focus manager.
  await page.evaluate(() =>
    document.dispatchEvent(new Event('visibilitychange')),
  )
  const refreshedSessionResponse = await refreshedSession
  expect(refreshedSessionResponse.ok()).toBe(true)
  expect((await refreshedSessionResponse.json()).user.email_verified).toBe(true)
  await expect(page.getByRole('link', { name: '+ Invitar' })).toBeEnabled()
  await inspectScreen(page)

  await page.goto('/settings')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Ajustes' }),
  ).toBeVisible()
  await page.getByRole('button', { name: 'Cerrar sesión' }).click()
  await expect(page).toHaveURL(/\/login\?reason=logged_out$/)
  await expect(page.getByText('Cerraste sesión.')).toBeVisible()
  await inspectScreen(page)
  await page.getByLabel('Email').fill(adminEmail)
  await page.getByLabel('Contraseña').fill(initialPassword)
  await page.getByRole('button', { name: 'Ingresar' }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: `Hola, ${adminName}` }),
  ).toBeVisible()
  await inspectScreen(page)

  const operatorEmail = `f702-operator-${randomUUID()}@example.test`
  const operatorName = 'Operador de prueba'
  const operatorPassword = 'Operator test password 123!'
  await page.goto('/settings/users')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Usuarios' }),
  ).toBeVisible()
  await inspectScreen(page)
  await page.getByRole('link', { name: '+ Invitar' }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: 'Invitar a alguien' }),
  ).toBeVisible()
  await inspectScreen(page)
  await page.getByLabel('Email').fill(operatorEmail)
  await page.getByRole('radio', { name: 'Operador' }).check()
  await page.getByRole('button', { name: 'Enviar invitación' }).click()
  await expect(page).toHaveURL(/\/settings\/users$/)
  await expect(
    page.getByText(`Invitación enviada a ${operatorEmail}.`, { exact: false }),
  ).toBeVisible()
  await inspectScreen(page)

  const operatorInvitation = await waitForEmail(
    'Invitación a tu empresa',
    operatorEmail,
  )
  const operatorContext = await browser.newContext({
    viewport: { width: 320, height: 720 },
  })
  const operatorPage = await operatorContext.newPage()
  await track(operatorPage)
  const operatorURL = extractEmailLink(operatorInvitation)
  expect(operatorURL.origin).toBe(origin)
  await operatorPage.goto(operatorURL.toString())
  await expect(
    operatorPage.getByRole('heading', {
      level: 2,
      name: `Te invitaron a ${companyName} como Operador`,
    }),
  ).toBeVisible()
  await inspectScreen(operatorPage)
  await operatorPage.getByLabel('Tu nombre').fill(operatorName)
  await operatorPage.getByLabel('Contraseña').fill(operatorPassword)
  await operatorPage.getByRole('button', { name: 'Aceptar invitación' }).click()
  await expect(
    operatorPage.getByRole('heading', {
      level: 1,
      name: `Hola, ${operatorName}`,
    }),
  ).toBeVisible()
  await inspectScreen(operatorPage)
  await operatorPage.goto('/settings/users')
  await expect(
    operatorPage.getByText('No tenés acceso a esta sección'),
  ).toBeVisible()
  await inspectScreen(operatorPage)

  const adminInviteeEmail = `f702-new-admin-${randomUUID()}@example.test`
  const adminInviteeName = 'Admin invitado'
  const adminInviteePassword = 'Invited admin password 123!'
  await page.goto('/settings/users')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Usuarios' }),
  ).toBeVisible()
  await page.getByRole('link', { name: '+ Invitar' }).click()
  await page.getByLabel('Email').fill(adminInviteeEmail)
  await page.getByRole('radio', { name: 'Operador' }).check()
  await page.getByRole('button', { name: 'Enviar invitación' }).click()
  await expect(page).toHaveURL(/\/settings\/users$/)
  const firstAdminInvite = await waitForEmail(
    'Invitación a tu empresa',
    adminInviteeEmail,
  )
  await page.getByRole('link', { name: '+ Invitar' }).click()
  await page.getByLabel('Email').fill(adminInviteeEmail)
  await page.getByRole('radio', { name: 'Administrador' }).check()
  await page.getByRole('button', { name: 'Enviar invitación' }).click()
  await expect(page).toHaveURL(/\/settings\/users$/)
  await expect(
    page.getByText(
      `${adminInviteeEmail} ya estaba invitado: le reenviamos la invitación como Administrador.`,
    ),
  ).toBeVisible()
  await inspectScreen(page)

  const updatedAdminInvite = await waitForEmail(
    'Invitación a tu empresa',
    adminInviteeEmail,
    firstAdminInvite.ID,
  )
  const invitedAdminContext = await browser.newContext({
    viewport: { width: 320, height: 720 },
  })
  const invitedAdminPage = await invitedAdminContext.newPage()
  await track(invitedAdminPage)
  const invitedAdminURL = extractEmailLink(updatedAdminInvite)
  expect(invitedAdminURL.origin).toBe(origin)
  await invitedAdminPage.goto(invitedAdminURL.toString())
  await expect(
    invitedAdminPage.getByRole('heading', {
      level: 2,
      name: `Te invitaron a ${companyName} como Administrador`,
    }),
  ).toBeVisible()
  await inspectScreen(invitedAdminPage)
  await invitedAdminPage.getByLabel('Tu nombre').fill(adminInviteeName)
  await invitedAdminPage.getByLabel('Contraseña').fill(adminInviteePassword)
  await invitedAdminPage
    .getByRole('button', { name: 'Aceptar invitación' })
    .click()
  await expect(
    invitedAdminPage.getByRole('heading', {
      level: 1,
      name: `Hola, ${adminInviteeName}`,
    }),
  ).toBeVisible()
  await invitedAdminPage.goto('/settings/users')
  await expect(
    invitedAdminPage.getByRole('heading', { level: 1, name: 'Usuarios' }),
  ).toBeVisible()
  await expect(
    invitedAdminPage.getByText('Administrador', { exact: true }),
  ).toBeVisible()
  await inspectScreen(invitedAdminPage)

  const operatorRow = page
    .getByRole('listitem')
    .filter({ hasText: operatorEmail })
  const operatorActions = operatorRow.getByRole('button', {
    name: `Acciones para ${operatorName}`,
  })
  // T-F707 (NFR-F07): icon-only controls keep the approved 44×44 CSS-pixel target.
  const operatorActionsBounds = await operatorActions.boundingBox()
  expect(operatorActionsBounds).not.toBeNull()
  expect(operatorActionsBounds!.width).toBeGreaterThanOrEqual(44)
  expect(operatorActionsBounds!.height).toBeGreaterThanOrEqual(44)
  await operatorActions.click()
  await page.getByRole('menuitem', { name: 'Desactivar' }).click()
  await expect(
    page.getByRole('alertdialog', { name: `¿Desactivar a ${operatorName}?` }),
  ).toBeVisible()
  await page
    .getByRole('alertdialog')
    .getByRole('button', { name: 'Desactivar' })
    .click()
  await expect(page.getByText(`Desactivaste a ${operatorName}`)).toBeVisible()
  await inspectScreen(page)
  await operatorPage.goto('/settings/users')
  await expect(
    operatorPage.getByText('Tu sesión se cerró. Ingresá de nuevo para seguir.'),
  ).toBeVisible()
  await inspectScreen(operatorPage)

  await page.goto('/settings')
  await page.getByRole('button', { name: 'Cerrar sesión' }).click()
  await expect(page).toHaveURL(/\/login\?reason=logged_out$/)
  await page.getByRole('link', { name: 'Registrá tu empresa' }).click()
  const recoveryEmail = `f702-recovery-${randomUUID()}@example.test`
  const recoveryName = 'Usuario para recuperar'
  const recoveryCompany = `Recuperación E2E ${randomUUID().slice(0, 8)}`
  await page.getByLabel('Tu nombre').fill(recoveryName)
  await page.getByLabel('Email').fill(recoveryEmail)
  await page.getByLabel('Contraseña').fill(initialPassword)
  await page.getByLabel('Nombre de la empresa').fill(recoveryCompany)
  await page.getByRole('radio', { name: 'Genérico' }).check()
  await page.getByRole('button', { name: 'Crear cuenta' }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: `Hola, ${recoveryName}` }),
  ).toBeVisible()
  await inspectScreen(page)

  await page.goto('/settings')
  await page.getByRole('button', { name: 'Cerrar sesión' }).click()
  await expect(page).toHaveURL(/\/login\?reason=logged_out$/)
  await page.getByRole('link', { name: 'Registrá tu empresa' }).click()
  await page.getByLabel('Tu nombre').fill(recoveryName)
  await page.getByLabel('Email').fill(recoveryEmail)
  await page.getByLabel('Contraseña').fill(initialPassword)
  await page.getByLabel('Nombre de la empresa').fill(recoveryCompany)
  await page.getByRole('radio', { name: 'Genérico' }).check()
  await page.getByRole('button', { name: 'Crear cuenta' }).click()
  await expect(
    page.getByText('Ya existe un usuario con ese email.'),
  ).toBeVisible()
  await inspectScreen(page)
  await page.getByRole('button', { name: 'Recuperar contraseña' }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: 'Recuperá tu contraseña' }),
  ).toBeVisible()
  await expect(page.getByLabel('Email')).toHaveValue(recoveryEmail)
  await inspectScreen(page)
  await page.getByRole('button', { name: 'Enviar enlace' }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: 'Revisá tu email' }),
  ).toBeVisible()
  await inspectScreen(page)

  const resetMessage = await waitForEmail(
    'Recuperá tu contraseña',
    recoveryEmail,
  )
  const resetURL = extractEmailLink(resetMessage)
  expect(resetURL.origin).toBe(origin)
  const resetPage = await page.context().newPage()
  await resetPage.setViewportSize({ width: 320, height: 720 })
  await track(resetPage)
  await resetPage.goto(resetURL.toString())
  await expect(
    resetPage.getByRole('heading', {
      level: 1,
      name: 'Creá una contraseña nueva',
    }),
  ).toBeVisible()
  await inspectScreen(resetPage)
  await resetPage.getByLabel('Nueva contraseña').fill(resetPassword)
  await resetPage.getByRole('button', { name: 'Guardar contraseña' }).click()
  await expect(resetPage).toHaveURL(/\/login\?reason=password_changed$/)
  await expect(
    resetPage.getByText(
      'Listo, cambiaste tu contraseña. Ingresá con la nueva.',
    ),
  ).toBeVisible()
  await inspectScreen(resetPage)
  await resetPage.getByLabel('Email').fill(recoveryEmail)
  await resetPage.getByLabel('Contraseña').fill(resetPassword)
  await resetPage.getByRole('button', { name: 'Ingresar' }).click()
  await expect(
    resetPage.getByRole('heading', { level: 1, name: `Hola, ${recoveryName}` }),
  ).toBeVisible()
  await expect(
    resetPage.getByText('Confirmá tu email', { exact: false }),
  ).toHaveCount(0)
  await inspectScreen(resetPage)

  const reopenedReset = await page.context().newPage()
  await reopenedReset.setViewportSize({ width: 320, height: 720 })
  await track(reopenedReset)
  await reopenedReset.goto(resetURL.toString())
  await expect(
    reopenedReset.getByText('Este enlace ya no sirve.'),
  ).toBeVisible()
  await inspectScreen(reopenedReset)

  await resetPage.goto('/settings')
  await resetPage.getByRole('button', { name: 'Cerrar sesión' }).click()
  await expect(resetPage).toHaveURL(/\/login\?reason=logged_out$/)
  await resetPage.getByLabel('Email').fill(recoveryEmail)
  for (let attempt = 1; attempt <= 5; attempt += 1) {
    await resetPage
      .getByLabel('Contraseña')
      .fill(`Incorrect password ${attempt}!`)
    await resetPage.getByRole('button', { name: 'Ingresar' }).click()
    await expect(
      resetPage.getByText('El email o la contraseña no son correctos.'),
    ).toBeVisible()
  }
  await inspectScreen(resetPage)
  await resetPage.getByLabel('Contraseña').fill('Sixth wrong password 123!')
  await resetPage.getByRole('button', { name: 'Ingresar' }).click()
  await expect(
    resetPage.getByText('Por seguridad, pausamos el ingreso con este email.'),
  ).toBeVisible()
  await expect(
    resetPage.getByText(
      /Podés volver a intentar a las .* o restablecer tu contraseña ahora\./,
    ),
  ).toBeVisible()
  await expect(
    resetPage.getByRole('button', { name: 'Restablecer contraseña' }),
  ).toBeVisible()
  await inspectScreen(resetPage)

  for (const entry of diagnostics) {
    expect(entry.policyViolations, 'CSP policy violations').toEqual([])
    expect(entry.consoleErrors, 'browser console errors').toEqual([])
    expect(entry.pageErrors, 'uncaught page errors').toEqual([])
  }
  await Promise.all(
    trackedPages
      .filter((target) => target !== page)
      .map((target) => target.context().close()),
  )
})
