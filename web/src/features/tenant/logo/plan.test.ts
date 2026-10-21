import { describe, expect, it } from 'vitest'
import {
  LOGO_INPUT_MAX_BYTES,
  LOGO_INPUT_MAX_PIXELS,
  LOGO_TARGET_MAX_BYTES,
} from './limits'
import { fitWithin, planLogoProcessing } from './plan'

const MIB = 1024 * 1024
const png = (width: number, height: number, byteSize: number) =>
  planLogoProcessing({ type: 'png', byteSize, width, height })
const jpeg = (width: number, height: number, byteSize: number) =>
  planLogoProcessing({ type: 'jpeg', byteSize, width, height })
const pngLadder = [2000, 1600, 1200, 1000, 700]

// T-F604 (DD-11, DD-31, DD-F21, DD-F22, INV-F13, ui.md §13.11.1)
describe('planLogoProcessing', () => {
  it('uploads a PNG inside the limits untouched', () => {
    expect(png(800, 400, 300 * 1024)).toEqual({ action: 'upload_original' })
  })
  it('keeps the byte limit inclusive, like the server', () => {
    expect(png(1000, 1000, 2_097_152)).toEqual({ action: 'upload_original' })
    expect(png(1000, 1000, 2_097_153)).toMatchObject({
      action: 'reencode',
      format: 'image/png',
    })
  })
  it('keeps the side limit inclusive', () => {
    expect(png(2000, 2000, 1000)).toEqual({ action: 'upload_original' })
    expect(png(2001, 100, 1000)).toMatchObject({ action: 'reencode' })
  })
  it('re-encodes an oversized PNG with the PNG ladder, never above the original side', () => {
    expect(png(3000, 3000, 1000)).toEqual({
      action: 'reencode',
      format: 'image/png',
      attempts: pngLadder.map((maxSide) => ({ maxSide })),
    })
    expect(png(1500, 1500, 2_500_000)).toEqual({
      action: 'reencode',
      format: 'image/png',
      attempts: [
        { maxSide: 1500 },
        { maxSide: 1200 },
        { maxSide: 1000 },
        { maxSide: 700 },
      ],
    })
  })
  it('always re-encodes a JPEG, starting at its own size with quality 0.90', () => {
    const plan = jpeg(1200, 800, 200 * 1024)
    expect(plan).toMatchObject({ action: 'reencode', format: 'image/jpeg' })
    expect(plan.action === 'reencode' && plan.attempts[0]).toEqual({
      maxSide: 1200,
      quality: 0.9,
    })
  })
  it('gives a big JPEG the complete ladder', () => {
    expect(jpeg(4000, 3000, 5 * MIB)).toEqual({
      action: 'reencode',
      format: 'image/jpeg',
      attempts: [
        { maxSide: 2000, quality: 0.9 },
        { maxSide: 2000, quality: 0.8 },
        { maxSide: 1600, quality: 0.8 },
        { maxSide: 1200, quality: 0.8 },
        { maxSide: 1000, quality: 0.75 },
      ],
    })
  })
  it('rejects what is too heavy to process, by pixels or by bytes', () => {
    const tooLarge = { action: 'reject', reason: 'too_large_to_process' }
    expect(png(5001, 5000, 1000)).toEqual(tooLarge)
    expect(jpeg(5001, 5000, 1000)).toEqual(tooLarge)
    expect(LOGO_INPUT_MAX_PIXELS).toBe(25_000_000)
    expect(png(5000, 5000, 1000)).toMatchObject({ action: 'reencode' })
    expect(jpeg(1000, 1000, LOGO_INPUT_MAX_BYTES + 1)).toEqual(tooLarge)
    expect(jpeg(1000, 1000, LOGO_INPUT_MAX_BYTES)).toMatchObject({
      action: 'reencode',
    })
  })
  it.each([
    ['svg', 'svg'],
    ['gif', 'unsupported_format'],
    ['webp', 'unsupported_format'],
    ['heic', 'unsupported_format'],
    ['unknown', 'unsupported_format'],
  ] as const)('rejects %s as %s', (type, reason) => {
    expect(
      planLogoProcessing({ type, byteSize: 100, width: null, height: null }),
    ).toEqual({ action: 'reject', reason })
  })
  it.each(['png', 'jpeg'] as const)(
    'rejects a %s whose size could not be read',
    (type) => {
      expect(
        planLogoProcessing({ type, byteSize: 100, width: null, height: null }),
      ).toEqual({ action: 'reject', reason: 'unreadable' })
    },
  )
})

describe('fitWithin', () => {
  it.each([
    [4000, 3000, 2000, { width: 2000, height: 1500 }],
    [3000, 4000, 2000, { width: 1500, height: 2000 }],
    [800, 400, 2000, { width: 800, height: 400 }],
  ])('(%i, %i, %i)', (width, height, maxSide, expected) => {
    expect(fitWithin(width, height, maxSide)).toEqual(expected)
  })
})

describe('limits', () => {
  it('matches the server byte limit exactly (DD-F22)', () => {
    expect(LOGO_TARGET_MAX_BYTES).toBe(2_097_152)
  })
})
