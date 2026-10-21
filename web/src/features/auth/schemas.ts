import { z } from 'zod'
import type {
  InvitationAcceptRequest,
  LoginRequest,
  PasswordResetConfirmRequest,
  PasswordResetRequest,
  SignupRequest,
} from '@/api/types'

const requiredText = (maximum: number) =>
  z.string().trim().min(1, 'required').max(maximum, 'too_long')

export const signupSchema = z
  .object({
    name: requiredText(120),
    email: z
      .string()
      .trim()
      .toLowerCase()
      .min(1, 'required')
      .max(254, 'too_long')
      .pipe(z.email({ error: 'invalid_format' })),
    password: z.string().min(10, 'too_short').max(128, 'too_long'),
    company_name: requiredText(120),
    base_currency: z.enum(['ARS', 'USD']),
    industry_template_code: z
      .string({ error: 'required' })
      .min(1, 'required')
      .max(64, 'too_long'),
    timezone: z.string().max(64, 'too_long').optional(),
  })
  .superRefine((values, context) => {
    if (values.password.toLowerCase() === values.email.toLowerCase()) {
      context.addIssue({
        code: 'custom',
        path: ['password'],
        message: 'same_as_email',
      })
    }
  }) satisfies z.ZodType<SignupRequest>

export type SignupFormValues = z.input<typeof signupSchema>

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

export const passwordResetConfirmSchema = z.object({
  token: z.string().min(1).max(128),
  password: z.string().min(10, 'too_short').max(128, 'too_long'),
}) satisfies z.ZodType<PasswordResetConfirmRequest>

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
