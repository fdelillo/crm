import { describe, expect, it, vi } from 'vitest'
import { jpegBytes, pngBytes, textBytes } from '@/test/imageBytes'
import { LOGO_TARGET_MAX_BYTES } from './limits'
import type {
  decodeImage as DecodeImage,
  encodeBitmap as EncodeBitmap,
} from './canvas'
import { prepareLogo } from './prepareLogo'

const bigJpeg = () =>
  new File(
    [jpegBytes({ width: 4000, height: 3000, size: 5_000_000 })],
    'foto.jpg',
    {
      type: 'image/jpeg',
    },
  )
const blobOf = (size: number) => new Blob([new Uint8Array(size)])

function fakes(sizes: number[]) {
  const close = vi.fn()
  const decodeImage = vi.fn<typeof DecodeImage>(() =>
    Promise.resolve({
      width: 4000,
      height: 3000,
      close,
    } as unknown as ImageBitmap),
  )
  const queue = [...sizes]
  const encodeBitmap = vi.fn<typeof EncodeBitmap>(() =>
    Promise.resolve(blobOf(queue.shift() ?? 3_000_000)),
  )
  return { close, decodeImage, encodeBitmap }
}

// T-F604 (DD-11, DD-31, DD-F20, DD-F21, DD-F22, INV-F13, H-11)
describe('prepareLogo', () => {
  it('returns the very same file for a PNG inside the limits, without touching the canvas', async () => {
    const file = new File([pngBytes(800, 400, 300_000)], 'logo.png', {
      type: 'image/png',
    })
    const deps = fakes([])
    const result = await prepareLogo(file, deps)
    expect(result).toEqual({ ok: true, file, resized: false, reencoded: false })
    expect(result.ok && result.file).toBe(file)
    expect(deps.decodeImage).not.toHaveBeenCalled()
    expect(deps.encodeBitmap).not.toHaveBeenCalled()
  })

  it('walks the JPEG ladder decoding once and stops at the first step that fits', async () => {
    const deps = fakes([2_400_000, 2_100_000, 1_500_000])
    const result = await prepareLogo(bigJpeg(), deps)
    expect(result.ok).toBe(true)
    if (!result.ok) return
    expect(result).toMatchObject({ resized: true, reencoded: true })
    expect(result.file.type).toBe('image/jpeg')
    expect(result.file.size).toBe(1_500_000)
    expect(deps.decodeImage).toHaveBeenCalledOnce()
    expect(deps.encodeBitmap.mock.calls.map(([, options]) => options)).toEqual([
      { maxSide: 2000, format: 'image/jpeg', quality: 0.9 },
      { maxSide: 2000, format: 'image/jpeg', quality: 0.8 },
      { maxSide: 1600, format: 'image/jpeg', quality: 0.8 },
    ])
  })

  it('accepts exactly 2 097 152 bytes and rejects one more', async () => {
    const deps = fakes([2_097_153, 2_097_152])
    const result = await prepareLogo(bigJpeg(), deps)
    expect(result.ok && result.file.size).toBe(2_097_152)
    expect(deps.encodeBitmap).toHaveBeenCalledTimes(2)
  })

  it('reports cannot_shrink when no step fits', async () => {
    const deps = fakes([])
    expect(await prepareLogo(bigJpeg(), deps)).toEqual({
      ok: false,
      reason: 'cannot_shrink',
    })
    expect(deps.encodeBitmap).toHaveBeenCalledTimes(5)
  })

  it('reports unreadable when decoding rejects', async () => {
    const deps = fakes([])
    deps.decodeImage.mockRejectedValueOnce(new Error('decode'))
    expect(await prepareLogo(bigJpeg(), deps)).toEqual({
      ok: false,
      reason: 'unreadable',
    })
    expect(deps.encodeBitmap).not.toHaveBeenCalled()
  })

  it('keeps going when one encoding step throws', async () => {
    const deps = fakes([])
    deps.encodeBitmap.mockRejectedValueOnce(new Error('toBlob'))
    deps.encodeBitmap.mockResolvedValueOnce(blobOf(1000))
    const result = await prepareLogo(bigJpeg(), deps)
    expect(result.ok).toBe(true)
  })

  it('closes the bitmap on every path that decoded', async () => {
    const success = fakes([1000])
    await prepareLogo(bigJpeg(), success)
    expect(success.close).toHaveBeenCalledOnce()
    const failure = fakes([])
    await prepareLogo(bigJpeg(), failure)
    expect(failure.close).toHaveBeenCalledOnce()
  })

  it('re-encodes an oversized PNG as PNG', async () => {
    const file = new File(
      [pngBytes(3000, 3000, LOGO_TARGET_MAX_BYTES + 1)],
      'logo.png',
      { type: 'image/png' },
    )
    const deps = fakes([1_000_000])
    const result = await prepareLogo(file, deps)
    expect(result.ok && result.file.type).toBe('image/png')
    expect(deps.encodeBitmap).toHaveBeenCalledWith(expect.anything(), {
      maxSide: 2000,
      format: 'image/png',
    })
  })

  it('does not call a re-encoded JPEG inside the limits "resized"', async () => {
    const small = new File(
      [jpegBytes({ width: 1200, height: 800, size: 200_000 })],
      'a.jpg',
    )
    const result = await prepareLogo(small, fakes([150_000]))
    expect(result).toMatchObject({ ok: true, resized: false, reencoded: true })
  })

  it.each([
    ['an SVG', textBytes('<svg></svg>'), 'svg'],
    ['plain text', textBytes('hola'), 'unsupported_format'],
    [
      'a PNG without a readable header',
      pngBytes(10, 10).slice(0, 12),
      'unreadable',
    ],
  ] as const)('rejects %s without decoding', async (_name, bytes, reason) => {
    const deps = fakes([])
    expect(await prepareLogo(new File([bytes], 'x'), deps)).toEqual({
      ok: false,
      reason,
    })
    expect(deps.decodeImage).not.toHaveBeenCalled()
  })
})
