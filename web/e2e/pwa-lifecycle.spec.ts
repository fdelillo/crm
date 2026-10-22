import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import type { SignupRequest } from '../src/api/types'

type WebAppManifest = {
  id: string
  name: string
  short_name: string
  lang: string
  start_url: string
  scope: string
  display: string
  background_color: string
  theme_color: string
  icons: { src: string; sizes: string; purpose: string }[]
}

// T-F703: validate the installed worker, its single offline cache, and session storage cleanup.
test('PWA serves only the offline page without retaining session data', async ({
  page,
}) => {
  test.setTimeout(120_000)

  const origin = 'https://localhost:8443'
  const email = `pwa-${randomUUID()}@example.test`
  const signup: SignupRequest = {
    name: 'Persona PWA de prueba',
    email,
    password: 'Fictional PWA password 123!',
    company_name: `Empresa PWA ${randomUUID().slice(0, 8)}`,
    base_currency: 'ARS',
    industry_template_code: 'generic',
    timezone: 'America/Argentina/Buenos_Aires',
  }
  const signupResponse = await page.request.post('/api/v1/auth/signup', {
    data: signup,
    headers: { Origin: origin },
  })
  expect(signupResponse.status()).toBe(201)

  await page.goto('/')
  await page.waitForFunction(
    () => navigator.serviceWorker.controller?.state === 'activated',
    undefined,
    { timeout: 20_000 },
  )
  const registration = await page.evaluate(async () => {
    const ready = await navigator.serviceWorker.ready
    return {
      scope: ready.scope,
      activeState: ready.active?.state,
    }
  })
  expect(registration).toEqual({
    scope: `${origin}/`,
    activeState: 'activated',
  })

  const manifestResponse = await page.request.get('/manifest.webmanifest')
  expect(manifestResponse.status()).toBe(200)
  const manifest = (await manifestResponse.json()) as WebAppManifest
  expect(manifest).toMatchObject({
    id: '/',
    name: 'CRM',
    short_name: 'CRM',
    lang: 'es-AR',
    start_url: '/',
    scope: '/',
    display: 'standalone',
    background_color: '#F8FAFC',
    theme_color: '#2563EB',
  })
  for (const icon of manifest.icons) {
    const response = await page.request.get(icon.src)
    expect(response.status(), icon.src).toBe(200)
  }
  expect(manifest.icons).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ sizes: '192x192', purpose: 'any' }),
      expect.objectContaining({ sizes: '512x512', purpose: 'any' }),
      expect.objectContaining({ sizes: '512x512', purpose: 'maskable' }),
    ]),
  )

  const cacheBeforeLogout = await page.evaluate(async () => {
    const names = await caches.keys()
    const entries = await Promise.all(
      names.map(async (name) => {
        const cache = await caches.open(name)
        return (await cache.keys()).map(
          (request) => new URL(request.url).pathname,
        )
      }),
    )
    return { names, entries: entries.flat() }
  })
  expect(cacheBeforeLogout).toEqual({
    names: ['crm-offline-v1'],
    entries: ['/offline.html'],
  })

  await page.goto('/settings')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Ajustes' }),
  ).toBeVisible()
  await page.getByRole('button', { name: 'Cerrar sesión' }).click()
  await expect(page).toHaveURL(/\/login\?reason=logged_out$/)
  const browserStorage = await page.evaluate(async () => {
    const storageEntries = (storage: Storage) =>
      Array.from({ length: storage.length }, (_, index) => {
        const key = storage.key(index)!
        return [key, storage.getItem(key) ?? '']
      })
    return {
      localStorage: storageEntries(localStorage),
      sessionStorage: storageEntries(sessionStorage),
      indexedDB: await indexedDB.databases(),
    }
  })
  expect(browserStorage.localStorage).toEqual([])
  expect(browserStorage.indexedDB).toEqual([])
  expect(browserStorage.sessionStorage).not.toContainEqual([
    'crm.verifyBanner.dismissed',
    'true',
  ])
  expect(
    JSON.stringify(browserStorage.sessionStorage).toLowerCase(),
  ).not.toContain(email.toLowerCase())
  expect(
    browserStorage.sessionStorage.every(
      ([key]) => key === 'crm.preloadReloaded',
    ),
    JSON.stringify(browserStorage.sessionStorage),
  ).toBe(true)

  await page.context().setOffline(true)
  await page.goto('/settings')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Sin conexión.' }),
  ).toBeVisible()
  await expect(
    page.getByText('Para usar la app necesitás internet.'),
  ).toBeVisible()
  await expect(page.getByRole('link', { name: 'Reintentar' })).toHaveAttribute(
    'href',
    '/',
  )

  const apiRequestFailed = await page.evaluate(async () => {
    try {
      await fetch('/api/v1/me')
      return false
    } catch {
      return true
    }
  })
  expect(apiRequestFailed).toBe(true)

  const cacheOffline = await page.evaluate(async () => {
    const names = await caches.keys()
    const entries = await Promise.all(
      names.map(async (name) => {
        const cache = await caches.open(name)
        return (await cache.keys()).map(
          (request) => new URL(request.url).pathname,
        )
      }),
    )
    return { names, entries: entries.flat() }
  })
  expect(cacheOffline).toEqual({
    names: ['crm-offline-v1'],
    entries: ['/offline.html'],
  })
})
