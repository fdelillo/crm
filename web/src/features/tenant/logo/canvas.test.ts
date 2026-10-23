import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  jpegBytes,
  jpegSegment,
  pngBytes,
  webkitExifSegment,
} from '@/test/imageBytes'
import { encodeBitmap } from './canvas'

// T-F707 (P-F25, DD-F36): jsdom has no canvas, so the browser's encoder is replaced at the
// HTMLCanvasElement prototype, the edge with the browser (like MSW at the network).
// withoutJpegMetadata is not replaced: what is checked is the Blob that comes out.
const bitmap = { width: 640, height: 480 } as ImageBitmap
const scan = [0x00, 0x0c, 0x12, 0x34, 0xff, 0x00]
const jfif = jpegSegment(
  0xe0,
  [...'JFIF'].map((c) => c.charCodeAt(0)).concat(0, 1, 1),
)

const jpegWithoutMetadata = () =>
  jpegBytes({ width: 640, height: 480, segments: [jfif], scan })
const jpegFromWebKit = () =>
  jpegBytes({
    width: 640,
    height: 480,
    segments: [jfif, webkitExifSegment(640, 480)],
    scan,
  })

// Replaces getContext and toBlob; `encoded` is what toBlob yields (null: it failed).
function stubBrowserEncoder(
  encoded: Uint8Array<ArrayBuffer> | null,
  type: string,
) {
  const canvases: HTMLCanvasElement[] = []
  const produced: Blob[] = []
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(
    () => ({ drawImage: () => {} }) as unknown as CanvasRenderingContext2D,
  )
  vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation(function (
    this: HTMLCanvasElement,
    callback: BlobCallback,
  ) {
    canvases.push(this)
    const blob = encoded ? new Blob([encoded], { type }) : null
    if (blob) produced.push(blob)
    callback(blob)
  })
  return { canvases, produced }
}

const bytesOfBlob = async (blob: Blob) =>
  new Uint8Array(await blob.arrayBuffer())

afterEach(() => {
  vi.restoreAllMocks()
})

describe('encodeBitmap', () => {
  it('returns a JPEG without the APP1 that WebKit on macOS writes', async () => {
    const { canvases, produced } = stubBrowserEncoder(
      jpegFromWebKit(),
      'image/jpeg',
    )

    const result = await encodeBitmap(bitmap, {
      maxSide: 2000,
      format: 'image/jpeg',
      quality: 0.9,
    })

    expect(result.type).toBe('image/jpeg')
    expect(result).not.toBe(produced[0])
    expect(await bytesOfBlob(result)).toEqual(jpegWithoutMetadata())
    expect(canvases[0]).toMatchObject({ width: 0, height: 0 })
  })

  it('returns the same Blob when the JPEG has no metadata (Chromium)', async () => {
    const { canvases, produced } = stubBrowserEncoder(
      jpegWithoutMetadata(),
      'image/jpeg',
    )

    const result = await encodeBitmap(bitmap, {
      maxSide: 2000,
      format: 'image/jpeg',
      quality: 0.9,
    })

    expect(result).toBe(produced[0])
    expect(canvases[0]).toMatchObject({ width: 0, height: 0 })
  })

  it('returns the same Blob for a PNG', async () => {
    const { canvases, produced } = stubBrowserEncoder(
      pngBytes(640, 480),
      'image/png',
    )

    const result = await encodeBitmap(bitmap, {
      maxSide: 2000,
      format: 'image/png',
    })

    expect(result).toBe(produced[0])
    expect(canvases[0]).toMatchObject({ width: 0, height: 0 })
  })

  it('rejects when the bytes of a JPEG cannot be walked', async () => {
    const { canvases } = stubBrowserEncoder(
      Uint8Array.from([1, 2, 3, 4]),
      'image/jpeg',
    )

    await expect(
      encodeBitmap(bitmap, {
        maxSide: 2000,
        format: 'image/jpeg',
        quality: 0.9,
      }),
    ).rejects.toThrow()
    expect(canvases[0]).toMatchObject({ width: 0, height: 0 })
  })

  it('rejects when toBlob gives null', async () => {
    const { canvases } = stubBrowserEncoder(null, 'image/jpeg')

    await expect(
      encodeBitmap(bitmap, {
        maxSide: 2000,
        format: 'image/jpeg',
        quality: 0.9,
      }),
    ).rejects.toThrow('Canvas encoding failed')
    expect(canvases[0]).toMatchObject({ width: 0, height: 0 })
  })
})
