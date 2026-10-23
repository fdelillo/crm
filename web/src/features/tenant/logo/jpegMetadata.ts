import { EOI, readJpegSegment, SOS, type JpegSegment } from './jpegSegments'

// Pure bytes in, bytes out: nothing from the browser (not even Blob). T-F707 (P-F25, DD-F36,
// INV-F13): a canvas JPEG carries what the encoder adds (WebKit on macOS writes an APP1/Exif),
// so before the first SOS only the segments that change how the image decodes are kept.
const ascii = (text: string) => [...text].map((char) => char.charCodeAt(0))
const KEPT_APP_SEGMENTS = new Map([
  [0xe0, ascii('JFIF\0')],
  [0xe2, ascii('ICC_PROFILE\0')],
  [0xee, ascii('Adobe')],
])

function isKept(bytes: Uint8Array, { marker, start, end }: JpegSegment) {
  const isMetadata = (marker >= 0xe0 && marker <= 0xef) || marker === 0xfe
  if (!isMetadata) return true
  const identifier = KEPT_APP_SEGMENTS.get(marker)
  // A closed list: any APPn or COM that is not named here goes, also future ones.
  return (
    identifier !== undefined &&
    start + 4 + identifier.length <= end &&
    identifier.every((value, index) => bytes[start + 4 + index] === value)
  )
}

/**
 * Drops the metadata segments before the first SOS: keeps APP0 'JFIF\0', APP2 'ICC_PROFILE\0'
 * and APP14 'Adobe'; every other APPn and every COM goes, and fill bytes are not kept. From the
 * first SOS on, the bytes are copied without looking. Returns the same instance when nothing
 * was dropped. Throws if the header cannot be walked to the SOS (no SOI, a length under 2 or
 * past the end, EOI or the end before the SOS, no FF where a marker goes): the caller must
 * never upload a JPEG it could not check.
 */
export function withoutJpegMetadata(bytes: Uint8Array): Uint8Array {
  if (bytes[0] !== 0xff || bytes[1] !== 0xd8)
    throw new Error('Not a JPEG: it does not start with SOI')
  const kept: [number, number][] = [[0, 2]]
  let changed = false
  let offset = 2
  for (;;) {
    const segment = readJpegSegment(bytes, offset)
    if (!segment || segment.end > bytes.length)
      throw new Error(`Malformed JPEG header at byte ${offset}`)
    if (segment.marker === SOS) {
      kept.push([segment.start, bytes.length])
      break
    }
    if (segment.marker === EOI)
      throw new Error('Malformed JPEG: EOI before the scan')
    const keep = isKept(bytes, segment)
    // Fill bytes before the marker are dropped too, so they count as a change.
    if (!keep || segment.start !== offset) changed = true
    if (keep) kept.push([segment.start, segment.end])
    offset = segment.end
  }
  if (!changed) return bytes
  const result = new Uint8Array(
    kept.reduce((total, [from, to]) => total + to - from, 0),
  )
  let written = 0
  for (const [from, to] of kept) {
    result.set(bytes.subarray(from, to), written)
    written += to - from
  }
  return result
}
