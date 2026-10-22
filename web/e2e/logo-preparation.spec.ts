import { createHash, randomUUID } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { expect, test, type Page } from '@playwright/test'

const jpegFixture = new URL('./fixtures/orientation-6-gps.jpg', import.meta.url)
const transparentPNGFixture = new URL(
  './fixtures/transparent-3000.png',
  import.meta.url,
)
const smallPNGFixture = new URL('./fixtures/small-800x400.png', import.meta.url)
const boundaryPNGFixture = new URL(
  './fixtures/boundary-2000px.png',
  import.meta.url,
)
const svgFixture = new URL('./fixtures/vector.svg', import.meta.url)
const gifFixture = new URL('./fixtures/animated.gif', import.meta.url)

type LogoFile = {
  name: string
  mimeType: string
  buffer: Buffer
}

type SavedLogo = {
  body: Buffer
  contentType: string | undefined
}

function logoRequest(response: {
  url(): string
  request(): { method(): string }
}): boolean {
  return (
    response.request().method() === 'GET' &&
    new URL(response.url()).pathname === '/api/v1/tenant/logo'
  )
}

async function chooseLogo(page: Page, file: LogoFile): Promise<void> {
  const chooserEvent = page.waitForEvent('filechooser')
  await page.getByText('Cambiar logo', { exact: true }).click()
  const chooser = await chooserEvent
  expect(chooser.isMultiple()).toBe(false)
  await chooser.setFiles(file)
}

function measurePreparation(page: Page): Promise<number> {
  return page.evaluate(
    () =>
      new Promise<number>((resolve, reject) => {
        const status = document.querySelector(
          '[aria-labelledby="company-logo-heading"] [role="status"]',
        )
        if (!status) {
          reject(new Error('Logo preparation status region is missing'))
          return
        }
        const startedAt = performance.now()
        let sawPreparing = false
        const observer = new MutationObserver(() => {
          const text = status.textContent ?? ''
          if (text.includes('Preparando la imagen…')) sawPreparing = true
          if (sawPreparing && text.includes('Subiendo logo…')) {
            window.clearTimeout(timeout)
            observer.disconnect()
            resolve(performance.now() - startedAt)
          }
        })
        const timeout = window.setTimeout(() => {
          observer.disconnect()
          reject(new Error('Logo did not move from preparation to upload'))
        }, 30_000)
        observer.observe(status, {
          childList: true,
          subtree: true,
          characterData: true,
        })
      }),
  )
}

async function uploadLogo(
  page: Page,
  file: LogoFile,
  measure = false,
): Promise<number | null> {
  const preparation = measure ? measurePreparation(page) : null
  const responseEvent = page.waitForResponse((response) => {
    return (
      response.request().method() === 'PUT' &&
      new URL(response.url()).pathname === '/api/v1/tenant/logo'
    )
  })
  await chooseLogo(page, file)
  const response = await responseEvent
  expect(response.status()).toBe(200)
  return preparation ? preparation : null
}

async function readSavedLogo(page: Page): Promise<SavedLogo> {
  const logo = page.getByRole('img', { name: /Logo de Empresa logo E2E/ })
  await expect(logo).toBeVisible()
  const src = await logo.getAttribute('src')
  expect(src).not.toBeNull()
  const requestKey = randomUUID()
  const responseEvent = page.waitForResponse((response) => {
    if (!logoRequest(response)) return false
    return new URL(response.url()).searchParams.get('e2e') === requestKey
  })
  const status = await page.evaluate(
    async ({ src, requestKey }) => {
      const url = new URL(src!, window.location.origin)
      url.searchParams.set('e2e', requestKey)
      const response = await fetch(url, { cache: 'no-store' })
      return response.status
    },
    { src, requestKey },
  )
  expect(status).toBe(200)
  const response = await responseEvent
  expect(response.status()).toBe(200)
  return {
    body: await response.body(),
    contentType: response.headers()['content-type'],
  }
}

async function inspectPixels(
  page: Page,
  body: Buffer,
  contentType: string,
): Promise<{ width: number; height: number; cornerAlpha: number }> {
  return page.evaluate(
    async ({ base64, contentType }) => {
      const binary = atob(base64)
      const bytes = Uint8Array.from(binary, (value) => value.charCodeAt(0))
      const bitmap = await createImageBitmap(
        new Blob([bytes], { type: contentType }),
      )
      const canvas = document.createElement('canvas')
      canvas.width = bitmap.width
      canvas.height = bitmap.height
      const context = canvas.getContext('2d', { willReadFrequently: true })
      if (!context) throw new Error('Canvas 2D context is unavailable')
      context.drawImage(bitmap, 0, 0)
      const cornerAlpha = context.getImageData(0, 0, 1, 1).data[3]
      if (cornerAlpha === undefined)
        throw new Error('Canvas did not return the corner pixel')
      const result = {
        width: bitmap.width,
        height: bitmap.height,
        cornerAlpha,
      }
      bitmap.close()
      return result
    },
    { base64: body.toString('base64'), contentType },
  )
}

function jpegExifDetails(bytes: Buffer): {
  hasExifApp1: boolean
  orientation: number | null
  hasGPSIFD: boolean
} {
  let offset = 2
  while (offset + 4 < bytes.length) {
    if (bytes[offset] !== 0xff) {
      offset += 1
      continue
    }
    while (bytes[offset] === 0xff) offset += 1
    const marker = bytes[offset]
    offset += 1
    if (marker === undefined) break
    if (marker === 0xda || marker === 0xd9) break
    if (marker === 0x01 || (marker >= 0xd0 && marker <= 0xd7)) continue
    if (offset + 2 > bytes.length) break
    const segmentLength = bytes.readUInt16BE(offset)
    if (segmentLength < 2 || offset + segmentLength > bytes.length) break
    const payload = offset + 2
    const isExif =
      marker === 0xe1 &&
      bytes.subarray(payload, payload + 6).equals(Buffer.from('Exif\0\0'))
    if (isExif) {
      const tiff = payload + 6
      const littleEndian = bytes.toString('ascii', tiff, tiff + 2) === 'II'
      const read16 = (at: number) =>
        littleEndian ? bytes.readUInt16LE(at) : bytes.readUInt16BE(at)
      const read32 = (at: number) =>
        littleEndian ? bytes.readUInt32LE(at) : bytes.readUInt32BE(at)
      const ifd0 = tiff + read32(tiff + 4)
      const entryCount = read16(ifd0)
      let orientation: number | null = null
      let hasGPSIFD = false
      for (let index = 0; index < entryCount; index += 1) {
        const entry = ifd0 + 2 + index * 12
        const tag = read16(entry)
        if (tag === 0x0112) orientation = read16(entry + 8)
        if (tag === 0x8825) hasGPSIFD = read32(entry + 8) > 0
      }
      return { hasExifApp1: true, orientation, hasGPSIFD }
    }
    offset += segmentLength
  }
  return { hasExifApp1: false, orientation: null, hasGPSIFD: false }
}

// T-F706 (DD-F20..22, S-F7, S-F9, NFR-F13): exercise real browser image APIs and saved bytes.
test('prepares company logos with real browser encoders', async ({
  page,
}, testInfo) => {
  test.setTimeout(240_000)
  const name = 'Persona logo de prueba'
  const companyName = `Empresa logo E2E ${randomUUID().slice(0, 8)}`
  const password = 'Fictional logo test password 123!'
  const email = `logo-${randomUUID()}@example.test`

  await page.goto('/signup')
  await page.getByLabel('Tu nombre').fill(name)
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Contraseña').fill(password)
  await page.getByLabel('Nombre de la empresa').fill(companyName)
  await page.getByRole('radio', { name: 'Genérico' }).check()
  await page.getByRole('button', { name: 'Crear cuenta' }).click()
  await expect(
    page.getByRole('heading', { level: 1, name: `Hola, ${name}` }),
  ).toBeVisible()

  await page.goto('/settings/company')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Datos de la empresa' }),
  ).toBeVisible()
  const logoRegion = page.getByRole('region', { name: 'Logo' })
  const logoStatus = logoRegion.getByRole('status')
  const fileInput = page.getByLabel('Cambiar logo')
  await expect(fileInput).toHaveAttribute('accept', 'image/png,image/jpeg')
  const putRequests: string[] = []
  page.on('request', (request) => {
    if (
      request.method() === 'PUT' &&
      new URL(request.url()).pathname === '/api/v1/tenant/logo'
    ) {
      putRequests.push(request.url())
    }
  })

  const jpeg = await readFile(jpegFixture)
  expect(jpeg.length).toBeGreaterThanOrEqual(4_500_000)
  expect(jpeg.length).toBeLessThanOrEqual(5_500_000)
  expect(jpegExifDetails(jpeg)).toMatchObject({
    hasExifApp1: true,
    orientation: 6,
    hasGPSIFD: true,
  })
  const jpegDuration = await uploadLogo(
    page,
    {
      name: 'orientation-6-gps.jpg',
      mimeType: 'image/jpeg',
      buffer: jpeg,
    },
    true,
  )
  const preparedJPEG = await readSavedLogo(page)
  const jpegPixels = await inspectPixels(page, preparedJPEG.body, 'image/jpeg')
  expect(preparedJPEG.contentType).toContain('image/jpeg')
  expect(preparedJPEG.body.length).toBeLessThanOrEqual(2_097_152)
  expect(jpegPixels.width).toBeLessThanOrEqual(2000)
  expect(jpegPixels.height).toBeLessThanOrEqual(2000)
  expect(jpegPixels.height).toBeGreaterThan(jpegPixels.width)
  expect(jpegExifDetails(preparedJPEG.body).hasExifApp1).toBe(false)
  testInfo.annotations.push({
    type: 'NFR-F13 JPEG preparation',
    description:
      `Tiempo hasta “Subiendo logo…”: ${jpegDuration?.toFixed(0)} ms; guardado ` +
      `${preparedJPEG.body.length} bytes ${jpegPixels.width}x${jpegPixels.height}, sin APP1/Exif.`,
  })

  const transparentPNG = await readFile(transparentPNGFixture)
  await uploadLogo(page, {
    name: 'transparent-3000.png',
    mimeType: 'image/png',
    buffer: transparentPNG,
  })
  const preparedPNG = await readSavedLogo(page)
  const pngPixels = await inspectPixels(page, preparedPNG.body, 'image/png')
  expect(preparedPNG.contentType).toContain('image/png')
  expect(preparedPNG.body.length).toBeLessThanOrEqual(2_097_152)
  expect(pngPixels.width).toBeLessThanOrEqual(2000)
  expect(pngPixels.height).toBeLessThanOrEqual(2000)
  expect(pngPixels.cornerAlpha).toBe(0)
  testInfo.annotations.push({
    type: 'PNG transparency',
    description:
      `Guardado ${preparedPNG.body.length} bytes ${pngPixels.width}x${pngPixels.height}; ` +
      `alpha esquina=${pngPixels.cornerAlpha}.`,
  })

  const smallPNG = await readFile(smallPNGFixture)
  expect(smallPNG.length).toBeLessThan(1_000_000)
  await uploadLogo(page, {
    name: 'small-800x400.png',
    mimeType: 'image/png',
    buffer: smallPNG,
  })
  const savedSmallPNG = await readSavedLogo(page)
  expect(savedSmallPNG.contentType).toContain('image/png')
  expect(savedSmallPNG.body.equals(smallPNG)).toBe(true)
  testInfo.annotations.push({
    type: 'PNG byte identity',
    description:
      `PNG 800x400: ${smallPNG.length} bytes, SHA-256 ` +
      `${createHash('sha256').update(smallPNG).digest('hex')}; bytes idénticos.`,
  })

  const boundaryPNG = await readFile(boundaryPNGFixture)
  expect(boundaryPNG.length).toBeGreaterThanOrEqual(1_900_000)
  expect(boundaryPNG.length).toBeLessThanOrEqual(2_097_152)
  await uploadLogo(page, {
    name: 'boundary-2000px.png',
    mimeType: 'image/png',
    buffer: boundaryPNG,
  })
  const savedBoundaryPNG = await readSavedLogo(page)
  expect(savedBoundaryPNG.contentType).toContain('image/png')
  expect(savedBoundaryPNG.body.length).toBe(boundaryPNG.length)
  expect(savedBoundaryPNG.body.equals(boundaryPNG)).toBe(true)
  testInfo.annotations.push({
    type: 'PNG exact limit',
    description:
      `PNG de ${boundaryPNG.length} bytes, SHA-256 ` +
      `${createHash('sha256').update(boundaryPNG).digest('hex')}; bytes idénticos.`,
  })

  const rejectedFormats = [
    {
      file: {
        name: 'vector.svg',
        mimeType: 'image/svg+xml',
        buffer: await readFile(svgFixture),
      },
      message:
        'No podemos usar logos en SVG. Exportalo como PNG desde el programa donde lo tenés y subilo de nuevo.',
    },
    {
      file: {
        name: 'animated.gif',
        mimeType: 'image/gif',
        buffer: await readFile(gifFixture),
      },
      message: 'El logo tiene que ser una imagen PNG o JPG.',
    },
  ]
  for (const { file, message } of rejectedFormats) {
    const priorRequests = putRequests.length
    await chooseLogo(page, file)
    await expect(logoRegion.getByRole('alert')).toContainText(message)
    expect(putRequests).toHaveLength(priorRequests)
  }
  await expect(logoStatus).toBeEmpty()
})
