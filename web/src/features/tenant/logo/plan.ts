import type { LogoRejectReason } from '@/api/errorMessages'
import {
  JPEG_LADDER,
  LOGO_INPUT_MAX_BYTES,
  LOGO_INPUT_MAX_PIXELS,
  LOGO_MAX_SIDE,
  LOGO_TARGET_MAX_BYTES,
  PNG_LADDER,
} from './limits'
import type { SniffedImageType } from './sniff'

export type { LogoRejectReason }
export interface EncodeAttempt {
  maxSide: number
  quality?: number
}
export type LogoPlan =
  | { action: 'upload_original' }
  | {
      action: 'reencode'
      format: 'image/png' | 'image/jpeg'
      attempts: EncodeAttempt[]
    }
  | { action: 'reject'; reason: LogoRejectReason }

interface PlanInput {
  type: SniffedImageType
  byteSize: number
  width: number | null
  height: number | null
}

/** True when the file would be refused by the server (DD-11, DD-31). */
export function exceedsServerLimits({
  byteSize,
  width,
  height,
}: Pick<PlanInput, 'byteSize' | 'width' | 'height'>): boolean {
  return (
    byteSize > LOGO_TARGET_MAX_BYTES ||
    Math.max(width ?? 0, height ?? 0) > LOGO_MAX_SIDE
  )
}

// Never enlarge, and skip steps that would repeat the previous one.
function ladder(
  steps: readonly EncodeAttempt[],
  longestSide: number,
): EncodeAttempt[] {
  const attempts: EncodeAttempt[] = []
  for (const step of steps) {
    const attempt = { ...step, maxSide: Math.min(step.maxSide, longestSide) }
    const last = attempts.at(-1)
    if (last?.maxSide !== attempt.maxSide || last.quality !== attempt.quality)
      attempts.push(attempt)
  }
  return attempts
}

/** T-F604 (DD-F20, DD-F21, ui.md §13.11.1): decides what to do with the chosen file. */
export function planLogoProcessing(input: PlanInput): LogoPlan {
  const { type, byteSize, width, height } = input
  if (byteSize > LOGO_INPUT_MAX_BYTES)
    return { action: 'reject', reason: 'too_large_to_process' }
  if (type === 'svg') return { action: 'reject', reason: 'svg' }
  if (type !== 'png' && type !== 'jpeg')
    return { action: 'reject', reason: 'unsupported_format' }
  if (width === null || height === null)
    return { action: 'reject', reason: 'unreadable' }
  if (width * height > LOGO_INPUT_MAX_PIXELS)
    return { action: 'reject', reason: 'too_large_to_process' }
  const longestSide = Math.max(width, height)
  if (type === 'png') {
    if (!exceedsServerLimits(input)) return { action: 'upload_original' }
    return {
      action: 'reencode',
      format: 'image/png',
      attempts: ladder(
        PNG_LADDER.map((maxSide) => ({ maxSide })),
        longestSide,
      ),
    }
  }
  // DD-F21: a JPEG is always re-encoded (EXIF orientation applied, metadata dropped).
  return {
    action: 'reencode',
    format: 'image/jpeg',
    attempts: ladder(JPEG_LADDER, longestSide),
  }
}

export function fitWithin(
  width: number,
  height: number,
  maxSide: number,
): { width: number; height: number } {
  const scale = Math.min(1, maxSide / Math.max(width, height))
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  }
}
