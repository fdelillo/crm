// DD-F22, ui.md §13.11.1: the client aims at exactly the server's limits (DD-11, DD-31).
// The server stays the authority; if it changes a limit, change it here in the same PR.
import type { EncodeAttempt } from './plan'

export const LOGO_MAX_SIDE = 2000
/** Exact server limit for the `file` part (LogoMaxBytes); compared with `<=`. */
export const LOGO_TARGET_MAX_BYTES = 2_097_152
/** Above this the phone is not asked to process the file at all. */
export const LOGO_INPUT_MAX_BYTES = 20 * 1024 * 1024
/** Decoding more than this takes over 100 MB of memory. */
export const LOGO_INPUT_MAX_PIXELS = 25_000_000

/** JPEG: lower the quality first, then the resolution. */
export const JPEG_LADDER: readonly Required<EncodeAttempt>[] = [
  { maxSide: 2000, quality: 0.9 },
  { maxSide: 2000, quality: 0.8 },
  { maxSide: 1600, quality: 0.8 },
  { maxSide: 1200, quality: 0.8 },
  { maxSide: 1000, quality: 0.75 },
]
/** PNG has no quality: only the resolution goes down. */
export const PNG_LADDER: readonly number[] = [2000, 1600, 1200, 1000, 700]
