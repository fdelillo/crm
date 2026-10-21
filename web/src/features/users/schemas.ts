import { z } from 'zod'
import type { InvitationRequest } from '@/api/types'

export const invitationSchema = z.object({
  email: z
    .string()
    .trim()
    .toLowerCase()
    .min(1, 'required')
    .max(254, 'too_long')
    .pipe(z.email({ error: 'invalid_format' })),
  role: z.enum(['admin', 'operator'], { error: 'invalid_value' }),
}) satisfies z.ZodType<InvitationRequest>

export type InvitationFormValues = z.input<typeof invitationSchema>
