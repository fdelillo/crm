import { describe, expect, it } from 'vitest'
import {
  gifBytes,
  heicBytes,
  jpegBytes,
  pngBytes,
  textBytes,
  webpBytes,
  withBom,
} from '@/test/imageBytes'
import { readImageSize, sniffImageType } from './sniff'

// T-F604 (DD-F20, INV-F13, ui.md §13.11.1): the format comes from the signature, never from the
// extension or from `file.type`.
describe('sniffImageType', () => {
  it('detects PNG by its signature', () => {
    expect(sniffImageType(pngBytes(10, 10))).toBe('png')
  })
  it.each([
    ['JFIF', 0xe0],
    ['EXIF', 0xe1],
  ])('detects JPEG with a %s first marker', (_name, first) => {
    expect(sniffImageType(jpegBytes({ width: 10, height: 10, first }))).toBe(
      'jpeg',
    )
  })
  it('detects GIF, WebP and HEIC', () => {
    expect(sniffImageType(gifBytes())).toBe('gif')
    expect(sniffImageType(webpBytes())).toBe('webp')
    expect(sniffImageType(heicBytes())).toBe('heic')
  })
  it('detects SVG text, also after a BOM and spaces and behind an XML prolog', () => {
    expect(sniffImageType(textBytes('<svg xmlns="x"></svg>'))).toBe('svg')
    expect(
      sniffImageType(withBom(textBytes('  \n\t<svg xmlns="x"></svg>'))),
    ).toBe('svg')
    expect(sniffImageType(textBytes('<?xml version="1.0"?><svg></svg>'))).toBe(
      'svg',
    )
  })
  it('trusts the declared SVG type when the bytes say nothing else', () => {
    expect(sniffImageType(textBytes('hola'), 'image/svg+xml')).toBe('svg')
  })
  it('does not let a declared SVG type turn real image bytes into SVG', () => {
    expect(sniffImageType(pngBytes(10, 10), 'image/svg+xml')).toBe('png')
  })
  it('returns unknown for arbitrary or empty bytes, whatever the declared type', () => {
    expect(sniffImageType(textBytes('esto no es una imagen'))).toBe('unknown')
    expect(sniffImageType(new Uint8Array())).toBe('unknown')
    expect(sniffImageType(textBytes('hola'), 'image/png')).toBe('unknown')
  })
})

describe('readImageSize', () => {
  it('reads a PNG IHDR', () => {
    expect(readImageSize(pngBytes(3000, 2000), 'png')).toEqual({
      width: 3000,
      height: 2000,
    })
  })
  it('skips a 20 KB EXIF segment before the baseline frame header', () => {
    expect(
      readImageSize(
        jpegBytes({ width: 4000, height: 3000, exifBytes: 20 * 1024 }),
        'jpeg',
      ),
    ).toEqual({ width: 4000, height: 3000 })
  })
  it('reads progressive JPEG (SOF2)', () => {
    expect(
      readImageSize(jpegBytes({ width: 1234, height: 567, sof: 0xc2 }), 'jpeg'),
    ).toEqual({ width: 1234, height: 567 })
  })
  it('does not mistake a DHT (C4) for a frame header', () => {
    expect(
      readImageSize(jpegBytes({ width: 800, height: 600, dht: true }), 'jpeg'),
    ).toEqual({ width: 800, height: 600 })
  })
  it('returns null for truncated headers', () => {
    expect(readImageSize(pngBytes(10, 10).slice(0, 20), 'png')).toBeNull()
    expect(
      readImageSize(jpegBytes({ width: 10, height: 10 }).slice(0, 8), 'jpeg'),
    ).toBeNull()
    expect(readImageSize(new Uint8Array(), 'jpeg')).toBeNull()
  })
  it('returns null when the scan starts before any frame header', () => {
    expect(
      readImageSize(Uint8Array.from([0xff, 0xd8, 0xff, 0xda, 0, 2]), 'jpeg'),
    ).toBeNull()
  })
})
