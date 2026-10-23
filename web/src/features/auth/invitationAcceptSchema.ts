import { z } from 'zod'
import type { InvitationAcceptRequest } from '@/api/types'

const requiredText = (maximum: number) =>
  z.string().trim().min(1, 'required').max(maximum, 'too_long')

export function invitationAcceptSchema(email: string) {
  return z
    .object({
      token: z.string().min(1).max(128),
      name: requiredText(120),
      password: z.string().min(10, 'too_short').max(128, 'too_long'),
    })
    .superRefine((values, context) => {
      if (values.password.toLowerCase() === email.toLowerCase()) {
        context.addIssue({
          code: 'custom',
          path: ['password'],
          message: 'same_as_email',
        })
      }
    }) satisfies z.ZodType<InvitationAcceptRequest>
}

export type InvitationAcceptFormValues = z.input<
  ReturnType<typeof invitationAcceptSchema>
>
