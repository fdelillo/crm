import { z } from 'zod'
import type { LoginRequest } from '@/api/types'

export const loginSchema = z.object({
  email: z
    .string()
    .trim()
    .toLowerCase()
    .min(1, 'required')
    .max(254, 'too_long')
    .pipe(z.email({ error: 'invalid_format' })),
  password: z.string().min(1, 'required').max(128, 'too_long'),
}) satisfies z.ZodType<LoginRequest>

export type LoginFormValues = z.input<typeof loginSchema>
