import { z } from 'zod'
import type { PasswordResetRequest } from '@/api/types'

export const passwordResetSchema = z.object({
  email: z
    .string()
    .trim()
    .toLowerCase()
    .min(1, 'required')
    .max(254, 'too_long')
    .pipe(z.email({ error: 'invalid_format' })),
}) satisfies z.ZodType<PasswordResetRequest>

export type PasswordResetFormValues = z.input<typeof passwordResetSchema>
