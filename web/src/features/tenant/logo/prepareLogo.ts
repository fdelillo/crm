import { decodeImage, encodeBitmap } from './canvas'
import { LOGO_TARGET_MAX_BYTES } from './limits'
import {
  exceedsServerLimits,
  planLogoProcessing,
  type LogoRejectReason,
} from './plan'
import { readImageSize, sniffImageType } from './sniff'

export type PrepareLogoResult =
  | { ok: true; file: File; resized: boolean; reencoded: boolean }
  | { ok: false; reason: LogoRejectReason }

// A JPEG can carry large EXIF and ICC segments before its frame header.
const HEADER_BYTES = 1024 * 1024

/**
 * T-F604 (DD-F20, ui.md §13.11.1): signature -> plan -> decode once -> ladder.
 * `resized` means the original broke the server limits and had to be shrunk;
 * a JPEG that was only re-encoded to drop its metadata is `reencoded` but not `resized`.
 */
export async function prepareLogo(
  file: File,
  deps: {
    decodeImage: typeof decodeImage
    encodeBitmap: typeof encodeBitmap
  } = { decodeImage, encodeBitmap },
): Promise<PrepareLogoResult> {
  const head = new Uint8Array(await file.slice(0, HEADER_BYTES).arrayBuffer())
  const type = sniffImageType(head, file.type)
  const size =
    type === 'png' || type === 'jpeg' ? readImageSize(head, type) : null
  const input = {
    type,
    byteSize: file.size,
    width: size?.width ?? null,
    height: size?.height ?? null,
  }
  const plan = planLogoProcessing(input)
  if (plan.action === 'reject') return { ok: false, reason: plan.reason }
  if (plan.action === 'upload_original')
    return { ok: true, file, resized: false, reencoded: false }

  let bitmap: ImageBitmap
  try {
    bitmap = await deps.decodeImage(file)
  } catch {
    return { ok: false, reason: 'unreadable' }
  }
  try {
    for (const attempt of plan.attempts) {
      let blob: Blob
      try {
        blob = await deps.encodeBitmap(bitmap, {
          maxSide: attempt.maxSide,
          format: plan.format,
          ...(attempt.quality === undefined
            ? {}
            : { quality: attempt.quality }),
        })
      } catch {
        // A step the browser could not encode counts as one that did not fit.
        continue
      }
      if (blob.size <= LOGO_TARGET_MAX_BYTES) {
        const extension = plan.format === 'image/png' ? 'png' : 'jpg'
        return {
          ok: true,
          file: new File([blob], `logo.${extension}`, { type: plan.format }),
          resized: exceedsServerLimits(input),
          reencoded: true,
        }
      }
    }
    return { ok: false, reason: 'cannot_shrink' }
  } finally {
    bitmap.close()
  }
}
