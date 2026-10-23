const u16 = (bytes: Uint8Array, offset: number) =>
  ((bytes[offset] ?? 0) << 8) | (bytes[offset + 1] ?? 0)

export const SOS = 0xda
export const EOI = 0xd9

// Markers that carry no length: TEM, RSTn, SOI.
const isStandalone = (marker: number) =>
  marker === 0x01 || (marker >= 0xd0 && marker <= 0xd8)

export interface JpegSegment {
  marker: number
  // Where the FF of the marker is (after any fill bytes) and where the next marker is
  // expected. `end` can lie past the data: the caller decides whether that is fatal.
  start: number
  end: number
}

/**
 * The segment that starts at `offset`, or null when no FF is there or the length is cut off
 * or smaller than 2. Fill bytes (FF FF) before the marker are skipped. SOS and EOI are
 * reported as two bytes: what follows is not a segment. Shared by the size reader (which
 * tolerates a bad header) and withoutJpegMetadata (which throws), T-F604 / T-F707.
 */
export function readJpegSegment(
  bytes: Uint8Array,
  offset: number,
): JpegSegment | null {
  if (bytes[offset] !== 0xff) return null
  let start = offset
  while (bytes[start + 1] === 0xff) start += 1
  const marker = bytes[start + 1]
  if (marker === undefined) return null
  if (isStandalone(marker) || marker === SOS || marker === EOI)
    return { marker, start, end: start + 2 }
  if (start + 4 > bytes.length) return null
  const length = u16(bytes, start + 2)
  if (length < 2) return null
  return { marker, start, end: start + 2 + length }
}
