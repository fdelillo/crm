import { describe, expect, it } from 'vitest'
import { jpegBytes, jpegSegment, webkitExifSegment } from '@/test/imageBytes'
import { withoutJpegMetadata } from './jpegMetadata'

// T-F707 (P-F25, DD-F36, INV-F13): byte tables for the closed list of segments that survive
// before the first SOS. The helpers build the headers; nothing here is a real image.
const ascii = (text: string) => [...text].map((char) => char.charCodeAt(0))
const jfif = jpegSegment(0xe0, [...ascii('JFIF'), 0, 1, 1, 0, 0, 1, 0, 1, 0, 0])
const jfxx = jpegSegment(0xe0, [...ascii('JFXX'), 0, 0x10, 1, 2, 3])
const xmp = jpegSegment(0xe1, [...ascii('http://ns.adobe.com/xap/1.0/'), 0, 60])
const icc = jpegSegment(0xe2, [...ascii('ICC_PROFILE'), 0, 1, 1, 9, 9])
const mpf = jpegSegment(0xe2, [...ascii('MPF'), 0, 7, 7, 7])
const photoshop = jpegSegment(0xed, [...ascii('Photoshop 3.0'), 0, 1, 2])
const app3 = jpegSegment(0xe3, [1, 2, 3])
const app15 = jpegSegment(0xef, [4, 5, 6])
const adobe = jpegSegment(0xee, [...ascii('Adobe'), 0, 100, 0, 0, 0, 0, 1])
const comment = jpegSegment(0xfe, ascii('made by a test'))
const dqt = jpegSegment(0xdb, [0, ...Array.from({ length: 64 }, () => 8)])
const dht = jpegSegment(0xc4, [0, ...Array.from({ length: 16 }, () => 1), 0])
const scan = [
  0x00, 0x0c, 3, 1, 0, 2, 0x11, 3, 0x11, 0, 63, 0, 0x12, 0x34, 0xff, 0x00,
]

const jpegWith = (...segments: number[][]) =>
  jpegBytes({ width: 640, height: 480, segments, scan })
const bytesOf = (...parts: (number[] | Uint8Array)[]) =>
  Uint8Array.from(parts.flatMap((part) => [...part]))
const SOI = [0xff, 0xd8]
const SOS = [0xff, 0xda]
const EOI = [0xff, 0xd9]

describe('withoutJpegMetadata', () => {
  it('drops the APP1 that WebKit on macOS writes and keeps everything else in place', () => {
    const exif = webkitExifSegment(640, 480)
    const input = jpegWith(jfif, exif, dqt, dht)
    const expected = jpegWith(jfif, dqt, dht)

    const output = withoutJpegMetadata(input)

    expect(exif).toHaveLength(78)
    expect(input.length - output.length).toBe(exif.length)
    expect(output).toEqual(expected)
    expect(output).not.toBe(input)
  })

  it.each([
    ['an XMP APP1', [xmp]],
    [
      'APP13 Photoshop, APP3, APP15 and a COM',
      [photoshop, app3, app15, comment],
    ],
    ['an APP2 MPF and an APP0 that is not JFIF', [mpf, jfxx]],
  ])('removes %s', (_name, removed) => {
    const output = withoutJpegMetadata(jpegWith(jfif, ...removed, dqt))

    expect(output).toEqual(jpegWith(jfif, dqt))
  })

  it('keeps the ICC profile and the Adobe segment, in their order', () => {
    const input = jpegWith(jfif, icc, adobe, dqt)

    expect(withoutJpegMetadata(input)).toBe(input)
  })

  it('keeps the ICC profile and the Adobe segment while removing what surrounds them', () => {
    const output = withoutJpegMetadata(
      jpegWith(webkitExifSegment(1, 1), icc, xmp, adobe, comment, dqt),
    )

    expect(output).toEqual(jpegWith(icc, adobe, dqt))
  })

  it('removes 20 APP1 segments of 65 533 bytes', () => {
    const input = jpegBytes({
      width: 640,
      height: 480,
      app1Segments: 20,
      segments: [jfif, dqt],
      scan,
    })

    const output = withoutJpegMetadata(input)

    expect(input.length - output.length).toBe(20 * 65_535)
    expect(output).toEqual(jpegWith(jfif, dqt))
  })

  it('returns the same instance when there is nothing to remove', () => {
    const input = jpegWith(jfif, dqt, dht)

    expect(withoutJpegMetadata(input)).toBe(input)
  })

  it('accepts fill bytes before a marker and does not keep them', () => {
    const input = bytesOf(
      SOI,
      [0xff, 0xff, 0xff],
      jfif,
      [0xff],
      dqt,
      [0xff, 0xff],
      SOS,
      scan,
    )

    expect(withoutJpegMetadata(input)).toEqual(
      bytesOf(SOI, jfif, dqt, SOS, scan),
    )
  })

  it('copies the bytes after the first SOS without looking at them', () => {
    const trailing = [
      0xff,
      0xe1,
      0x00,
      0x10,
      ...Array.from({ length: 14 }, () => 1),
    ]
    const input = bytesOf(SOI, jfif, dqt, SOS, scan, trailing, EOI)

    expect(withoutJpegMetadata(input)).toBe(input)
    const withExif = bytesOf(SOI, webkitExifSegment(1, 1), SOS, scan, trailing)
    expect(withoutJpegMetadata(withExif)).toEqual(
      bytesOf(SOI, SOS, scan, trailing),
    )
  })

  it.each([
    ['it does not start with SOI', bytesOf([0xff, 0xe0, 0, 2], SOS)],
    ['it is empty', new Uint8Array()],
    [
      'a segment length goes past the end',
      bytesOf(SOI, [0xff, 0xe1, 0x00, 0x40, 1, 2, 3]),
    ],
    ['a length is smaller than 2', bytesOf(SOI, [0xff, 0xe1, 0x00, 0x01], SOS)],
    ['an EOI comes before the SOS', bytesOf(SOI, jfif, EOI)],
    ['it ends without a SOS', bytesOf(SOI, jfif, dqt)],
    [
      'a byte other than FF is where a marker goes',
      bytesOf(SOI, jfif, [0x12, 0xdb, 0, 2], SOS),
    ],
  ])('throws when %s', (_name, input) => {
    expect(() => withoutJpegMetadata(input)).toThrow()
  })
})
