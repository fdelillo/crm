import { withoutJpegMetadata } from './jpegMetadata'
import { fitWithin } from './plan'

// The only module that touches createImageBitmap and <canvas> (DD-F20, lint-enforced).
// jsdom has neither, so screen tests replace this module; the real one runs in Playwright.

/** Decodes once, applying the EXIF orientation to the pixels. */
export function decodeImage(file: Blob): Promise<ImageBitmap> {
  return createImageBitmap(file, { imageOrientation: 'from-image' })
}

// P-F25, DD-F36: the encoder may add its own metadata (WebKit on macOS writes an APP1/Exif).
// It runs here, before the size is compared, so what is measured is what gets uploaded. A
// JPEG that cannot be walked rejects: it never goes up as it came.
async function withoutEncoderMetadata(blob: Blob): Promise<Blob> {
  const bytes = new Uint8Array(await blob.arrayBuffer())
  const stripped = withoutJpegMetadata(bytes)
  return stripped === bytes
    ? blob
    : new Blob([new Uint8Array(stripped)], { type: blob.type })
}

export async function encodeBitmap(
  bitmap: ImageBitmap,
  options: {
    maxSide: number
    format: 'image/png' | 'image/jpeg'
    quality?: number
  },
): Promise<Blob> {
  const { width, height } = fitWithin(
    bitmap.width,
    bitmap.height,
    options.maxSide,
  )
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const context = canvas.getContext('2d')
  if (!context) throw new Error('Canvas 2D is not available')
  // No background is painted: a PNG keeps its transparency.
  context.imageSmoothingQuality = 'high'
  context.drawImage(bitmap, 0, 0, width, height)
  try {
    const blob = await new Promise<Blob>((resolve, reject) =>
      canvas.toBlob(
        (blob) =>
          blob ? resolve(blob) : reject(new Error('Canvas encoding failed')),
        options.format,
        options.quality,
      ),
    )
    return options.format === 'image/jpeg'
      ? await withoutEncoderMetadata(blob)
      : blob
  } finally {
    // Safari keeps canvas backing stores alive; release this one right away.
    canvas.width = 0
    canvas.height = 0
  }
}
