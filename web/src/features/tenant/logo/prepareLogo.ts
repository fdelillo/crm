import { decodeImage, encodeBitmap } from './canvas'
import { LOGO_INPUT_MAX_BYTES, LOGO_TARGET_MAX_BYTES } from './limits'
import {
  exceedsServerLimits,
  planLogoProcessing,
  type LogoRejectReason,
} from './plan'
import { readImageSize, sniffImageType } from './sniff'

export type PrepareLogoResult =
  | { ok: true; file: File; resized: boolean; reencoded: boolean }
  | { ok: false; reason: LogoRejectReason }

/**
 * T-F604 (DD-F20, ui.md §13.11.1, P-F21): size -> read the whole file -> signature -> plan ->
 * decode once -> ladder. `resized` means something had to be shrunk: the original broke the
 * server limits, or the first step of the ladder did not fit. A JPEG that was only re-encoded
 * to drop its metadata is `reencoded` but not `resized`.
 */
export async function prepareLogo(
  file: File,
  deps: {
    decodeImage: typeof decodeImage
    encodeBitmap: typeof encodeBitmap
  } = { decodeImage, encodeBitmap },
): Promise<PrepareLogoResult> {
  // Looked at before reading anything: a huge file is never loaded. Past that, the whole file
  // is read, like the server does with image.DecodeConfig (a JPEG may carry over 1 MiB of
  // metadata before its frame header).
  if (file.size > LOGO_INPUT_MAX_BYTES)
    return { ok: false, reason: 'too_large_to_process' }
  const bytes = new Uint8Array(await file.arrayBuffer())
  const type = sniffImageType(bytes, file.type)
  const size =
    type === 'png' || type === 'jpeg' ? readImageSize(bytes, type) : null
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
    for (const [index, attempt] of plan.attempts.entries()) {
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
          resized: exceedsServerLimits(input) || index > 0,
          reencoded: true,
        }
      }
    }
    return { ok: false, reason: 'cannot_shrink' }
  } finally {
    bitmap.close()
  }
}
