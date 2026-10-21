import { fitWithin } from './plan'

// The only module that touches createImageBitmap and <canvas> (DD-F20, lint-enforced).
// jsdom has neither, so screen tests replace this module; the real one runs in Playwright.

/** Decodes once, applying the EXIF orientation to the pixels. */
export function decodeImage(file: Blob): Promise<ImageBitmap> {
  return createImageBitmap(file, { imageOrientation: 'from-image' })
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
    return await new Promise<Blob>((resolve, reject) =>
      canvas.toBlob(
        (blob) =>
          blob ? resolve(blob) : reject(new Error('Canvas encoding failed')),
        options.format,
        options.quality,
      ),
    )
  } finally {
    // Safari keeps canvas backing stores alive; release this one right away.
    canvas.width = 0
    canvas.height = 0
  }
}
