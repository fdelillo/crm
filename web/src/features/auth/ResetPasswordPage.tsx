import { useRef, useState } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { Link } from 'react-router'
import { AppBrand } from '@/components/AppBrand'
import { FormAlert } from '@/components/FormAlert'
import { PageHeader } from '@/components/PageHeader'
import { PasswordInput } from '@/components/PasswordInput'
import { SubmitButton } from '@/components/SubmitButton'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import {
  applyServerFieldErrors,
  fieldErrorText,
  messageForError,
} from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import { useConfirmPasswordReset } from './api'
import { passwordResetConfirmSchema } from './schemas'
import { useLinkToken } from './useLinkToken'

interface ResetPasswordValues {
  password: string
}

export function ResetPasswordPage() {
  const token = useLinkToken()
  const confirmReset = useConfirmPasswordReset()
  const submitLock = useRef(false)
  const [submitError, setSubmitError] = useState<unknown>(null)
  const form = useForm<ResetPasswordValues>({
    resolver: zodResolver(passwordResetConfirmSchema.omit({ token: true })),
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    defaultValues: { password: '' },
  })
  const {
    register,
    handleSubmit,
    setFocus,
    clearErrors,
    setError,
    formState: { errors, isSubmitting },
  } = form

  async function onSubmit(values: ResetPasswordValues) {
    if (!token || submitLock.current) return
    submitLock.current = true
    setSubmitError(null)
    try {
      await confirmReset.mutateAsync(
        passwordResetConfirmSchema.parse({ ...values, token }),
      )
    } catch (error) {
      if (isApiError(error) && error.code === 'validation_failed') {
        const unknownErrors = applyServerFieldErrors(
          setError,
          error.fieldErrors,
          ['password'],
        )
        const hasPasswordError = error.fieldErrors.some(
          (fieldError) => fieldError.field === 'password',
        )
        if (hasPasswordError) setFocus('password')
        if (error.fieldErrors.length > 0 && unknownErrors.length === 0) return
      }
      setSubmitError(error)
    } finally {
      submitLock.current = false
    }
  }

  const error = submitError
    ? messageForError(submitError, 'resetConfirm')
    : null
  const invalidToken =
    isApiError(submitError) && submitError.code === 'token_invalid'
  function submitForm(event: React.FormEvent<HTMLFormElement>) {
    void handleSubmit(onSubmit)(event)
  }
  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-center px-4 py-8 sm:px-6">
      <div className="mb-6">
        <AppBrand name="CRM" logoSrc={null} />
      </div>
      <section className="space-y-6 rounded-xl border bg-card p-5 shadow-sm sm:p-8">
        <PageHeader title="Creá una contraseña nueva" />
        {!token ? (
          <>
            <FormAlert
              variant="warning"
              title="Este enlace está incompleto."
              description="Abrilo de nuevo desde el email o pedí uno nuevo."
            />
            <Link
              className="inline-flex min-h-11 items-center text-primary underline"
              to="/forgot-password"
            >
              Pedir un enlace nuevo
            </Link>
          </>
        ) : invalidToken ? (
          <>
            <FormAlert
              variant="error"
              title={error?.title ?? 'Este enlace ya no sirve.'}
              description={error?.description}
            />
            <Link
              className="inline-flex min-h-11 items-center text-primary underline"
              to="/forgot-password"
            >
              Pedir un enlace nuevo
            </Link>
          </>
        ) : (
          <form noValidate className="space-y-5" onSubmit={submitForm}>
            <Field data-invalid={Boolean(errors.password)}>
              <FieldLabel htmlFor="new-password">Nueva contraseña</FieldLabel>
              <PasswordInput
                id="new-password"
                className="h-11"
                autoComplete="new-password"
                readOnly={confirmReset.isPending}
                aria-invalid={Boolean(errors.password)}
                aria-describedby={
                  errors.password ? 'new-password-error' : undefined
                }
                {...register('password', {
                  onChange: () => {
                    clearErrors('password')
                    setSubmitError(null)
                  },
                })}
              />
              {errors.password ? (
                <FieldError id="new-password-error">
                  {fieldErrorText('password', errors.password)}
                </FieldError>
              ) : (
                <p className="text-sm text-muted-foreground">
                  Mínimo 10 caracteres.
                </p>
              )}
            </Field>
            <p>
              Al guardarla se cierran tus sesiones abiertas en otros
              dispositivos.
            </p>
            {error && (
              <FormAlert
                variant="error"
                title={error.title}
                description={error.description}
                requestId={error.requestId}
                action={
                  error.retryable
                    ? {
                        label: 'Reintentar',
                        onClick: () => void handleSubmit(onSubmit)(),
                      }
                    : undefined
                }
              />
            )}
            <SubmitButton
              pending={confirmReset.isPending || isSubmitting}
              pendingLabel="Guardando…"
              className="h-11 w-full"
            >
              Guardar contraseña
            </SubmitButton>
          </form>
        )}
      </section>
    </main>
  )
}
