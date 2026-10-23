// Hand-made headers for the logo tests: just enough bytes for the sniffing and the size
// readers, never a real image. `size` pads the rest with zeros to reach an exact byte count.
type Bytes = Uint8Array<ArrayBuffer>
function pad(bytes: number[], size?: number): Bytes {
  if (size !== undefined && size < bytes.length)
    throw new Error(`size ${size} is smaller than the header`)
  const result = new Uint8Array(size ?? bytes.length)
  result.set(bytes)
  return result
}
const u16 = (value: number) => [(value >> 8) & 0xff, value & 0xff]
const u32 = (value: number) => [
  (value >>> 24) & 0xff,
  (value >> 16) & 0xff,
  (value >> 8) & 0xff,
  value & 0xff,
]
const ascii = (text: string) => [...text].map((char) => char.charCodeAt(0))

export const PNG_SIGNATURE = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]

export function pngBytes(width: number, height: number, size?: number) {
  return pad(
    [
      ...PNG_SIGNATURE,
      ...u32(13),
      ...ascii('IHDR'),
      ...u32(width),
      ...u32(height),
      8,
      6,
      0,
      0,
      0,
      ...u32(0),
    ],
    size,
  )
}

export function jpegBytes(options: {
  width: number
  height: number
  // Second byte of the start-of-frame marker: 0xC0 baseline, 0xC2 progressive.
  sof?: number
  // An APP1 (EXIF) segment of this many payload bytes before the frame header.
  exifBytes?: number
  // This many full APP1 segments (65 533 payload bytes each) before the frame header.
  app1Segments?: number
  // A DHT (0xFFC4) segment before the frame header.
  dht?: boolean
  // Whole segments (marker included, see jpegSegment) placed before the frame header.
  segments?: number[][]
  // Bytes written after the SOS marker, standing for the entropy-coded data.
  scan?: number[]
  // First marker after SOI: 0xE0 (JFIF) or 0xE1 (EXIF).
  first?: number
  size?: number
}) {
  const {
    width,
    height,
    sof = 0xc0,
    exifBytes,
    app1Segments = 0,
    dht,
    segments = [],
    scan = [],
    first,
    size,
  } = options
  const bytes = [0xff, 0xd8]
  if (first !== undefined) bytes.push(0xff, first, ...u16(2))
  if (exifBytes !== undefined) {
    bytes.push(0xff, 0xe1, ...u16(exifBytes + 2))
    for (let i = 0; i < exifBytes; i++) bytes.push(0)
  }
  for (let segment = 0; segment < app1Segments; segment++) {
    bytes.push(0xff, 0xe1, ...u16(0xffff))
    for (let i = 0; i < 0xffff - 2; i++) bytes.push(0)
  }
  if (dht) {
    // A Huffman table whose bytes would look like dimensions if mistaken for a frame header.
    bytes.push(0xff, 0xc4, ...u16(19), 0x00, ...u16(0x0bb8), ...u16(0x07d0))
    for (let i = 0; i < 12; i++) bytes.push(0x01)
  }
  for (const segment of segments) bytes.push(...segment)
  bytes.push(0xff, sof, ...u16(17), 8, ...u16(height), ...u16(width), 3)
  bytes.push(1, 0x22, 0, 2, 0x11, 1, 3, 0x11, 1)
  bytes.push(0xff, 0xda, ...scan)
  return pad(bytes, size)
}

/** One marker segment: FF, marker, the length (which counts itself) and the payload. */
export const jpegSegment = (marker: number, payload: number[]) => [
  0xff,
  marker,
  ...u16(payload.length + 2),
  ...payload,
]

/**
 * The APP1 that WebKit on macOS (ImageIO) writes into canvas.toBlob JPEGs, dumped from the
 * logo saved by the E2E (P-F25, DD-F36): 76 bytes, big-endian TIFF, an IFD0 whose only entry
 * points at an Exif IFD of three entries (ColorSpace = 1, width, height). No photo data.
 */
export const webkitExifSegment = (width: number, height: number) => [
  0xff,
  0xe1,
  0x00,
  0x4c,
  ...ascii('Exif'),
  0,
  0,
  ...ascii('MM'),
  0x00,
  0x2a,
  ...u32(8),
  ...u16(1),
  0x87,
  0x69,
  0x00,
  0x04,
  ...u32(1),
  ...u32(0x1a),
  ...u32(0),
  ...u16(3),
  0xa0,
  0x01,
  0x00,
  0x03,
  ...u32(1),
  0x00,
  0x01,
  0x00,
  0x00,
  0xa0,
  0x02,
  0x00,
  0x04,
  ...u32(1),
  ...u32(width),
  0xa0,
  0x03,
  0x00,
  0x04,
  ...u32(1),
  ...u32(height),
  ...u32(0),
]

export const gifBytes = () => pad([...ascii('GIF89a'), 1, 0, 1, 0])
export const webpBytes = () =>
  pad([...ascii('RIFF'), 0, 0, 0, 0, ...ascii('WEBPVP8 ')])
export const heicBytes = () =>
  pad([0, 0, 0, 24, ...ascii('ftypheic'), 0, 0, 0, 0])
export const textBytes = (text: string) => new TextEncoder().encode(text)
export const withBom = (bytes: Bytes): Bytes =>
  Uint8Array.from([0xef, 0xbb, 0xbf, ...bytes])
