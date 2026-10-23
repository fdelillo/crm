import { z } from 'zod'
import type { PasswordResetConfirmRequest } from '@/api/types'

export const passwordResetConfirmSchema = z.object({
  token: z.string().min(1).max(128),
  password: z.string().min(10, 'too_short').max(128, 'too_long'),
}) satisfies z.ZodType<PasswordResetConfirmRequest>
