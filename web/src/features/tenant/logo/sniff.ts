export type SniffedImageType =
  'png' | 'jpeg' | 'gif' | 'webp' | 'heic' | 'svg' | 'unknown'

const PNG = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]
const startsWith = (bytes: Uint8Array, signature: number[], offset = 0) =>
  signature.every((value, index) => bytes[offset + index] === value)
const ascii = (text: string) => [...text].map((char) => char.charCodeAt(0))
// Enough to see past a BOM and some blank space.
const SVG_PEEK_BYTES = 1024

function looksLikeSvg(bytes: Uint8Array): boolean {
  // TextDecoder drops a leading BOM by default.
  const text = new TextDecoder()
    .decode(bytes.subarray(0, SVG_PEEK_BYTES))
    .trimStart()
    .toLowerCase()
  return text.startsWith('<?xml') || text.startsWith('<svg')
}

/** Format by signature, never by extension (T-F604, ui.md §13.11.1). */
export function sniffImageType(
  bytes: Uint8Array,
  declaredType?: string,
): SniffedImageType {
  if (startsWith(bytes, PNG)) return 'png'
  if (startsWith(bytes, [0xff, 0xd8, 0xff])) return 'jpeg'
  if (startsWith(bytes, ascii('GIF8'))) return 'gif'
  if (startsWith(bytes, ascii('RIFF')) && startsWith(bytes, ascii('WEBP'), 8))
    return 'webp'
  if (startsWith(bytes, ascii('ftyp'), 4)) return 'heic'
  if (declaredType === 'image/svg+xml' || looksLikeSvg(bytes)) return 'svg'
  return 'unknown'
}

const u16 = (bytes: Uint8Array, offset: number) =>
  ((bytes[offset] ?? 0) << 8) | (bytes[offset + 1] ?? 0)
const u32 = (bytes: Uint8Array, offset: number) =>
  ((bytes[offset] ?? 0) * 0x1000000 +
    (((bytes[offset + 1] ?? 0) << 16) |
      ((bytes[offset + 2] ?? 0) << 8) |
      (bytes[offset + 3] ?? 0))) >>>
  0

function pngSize(bytes: Uint8Array) {
  if (bytes.length < 24 || !startsWith(bytes, ascii('IHDR'), 12)) return null
  return { width: u32(bytes, 16), height: u32(bytes, 20) }
}

// Start-of-frame markers are C0-CF except DHT (C4), JPG (C8) and DAC (CC).
const isFrameMarker = (marker: number) =>
  marker >= 0xc0 && marker <= 0xcf && ![0xc4, 0xc8, 0xcc].includes(marker)
// Markers that carry no length: TEM, RSTn, SOI.
const isStandalone = (marker: number) =>
  marker === 0x01 || (marker >= 0xd0 && marker <= 0xd8)

function jpegSize(bytes: Uint8Array) {
  let offset = 2
  while (offset + 4 <= bytes.length) {
    if (bytes[offset] !== 0xff) return null
    const marker = bytes[offset + 1] ?? 0
    if (marker === 0xff) {
      offset += 1
      continue
    }
    if (isStandalone(marker)) {
      offset += 2
      continue
    }
    // Reaching the scan (or the end) before any frame header: nothing to read.
    if (marker === 0xda || marker === 0xd9) return null
    if (isFrameMarker(marker)) {
      if (offset + 9 > bytes.length) return null
      const height = u16(bytes, offset + 5)
      const width = u16(bytes, offset + 7)
      return width > 0 && height > 0 ? { width, height } : null
    }
    offset += 2 + u16(bytes, offset + 2)
  }
  return null
}

/** Raw header dimensions (no EXIF rotation), without decoding the image. */
export function readImageSize(
  bytes: Uint8Array,
  type: 'png' | 'jpeg',
): { width: number; height: number } | null {
  return type === 'png' ? pngSize(bytes) : jpegSize(bytes)
}
