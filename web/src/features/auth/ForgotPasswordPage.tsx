import { useEffect, useRef, useState } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { Link, useLocation } from 'react-router'
import { AppBrand } from '@/components/AppBrand'
import { FormAlert } from '@/components/FormAlert'
import { PageHeader } from '@/components/PageHeader'
import { SubmitButton } from '@/components/SubmitButton'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { fieldErrorText, messageForError } from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import { useRequestPasswordReset } from './api'
import { passwordResetSchema, type PasswordResetFormValues } from './schemas'

export function ForgotPasswordPage() {
  const location = useLocation()
  const requestReset = useRequestPasswordReset()
  const submitLock = useRef(false)
  const [sent, setSent] = useState(false)
  const [submitError, setSubmitError] = useState<unknown>(null)
  const heading = useRef<HTMLHeadingElement>(null)
  const entryState = location.state as { email?: string } | null
  const form = useForm<PasswordResetFormValues>({
    resolver: zodResolver(passwordResetSchema),
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    defaultValues: { email: entryState?.email ?? '' },
  })
  const {
    register,
    handleSubmit,
    setFocus,
    clearErrors,
    setError,
    formState: { errors, isSubmitting },
  } = form

  useEffect(() => {
    if (sent) heading.current?.focus()
  }, [sent])

  async function onSubmit(values: PasswordResetFormValues) {
    if (submitLock.current) return
    submitLock.current = true
    setSubmitError(null)
    try {
      await requestReset.mutateAsync(passwordResetSchema.parse(values))
      setSent(true)
    } catch (error) {
      if (isApiError(error) && error.code === 'validation_failed') {
        const field = error.fieldErrors.find((item) => item.field === 'email')
        if (field) {
          setError('email', {
            type: 'server',
            message: fieldErrorText('email', {
              type: 'server',
              message: field.code,
            }),
          })
          setFocus('email')
          return
        }
      }
      setSubmitError(error)
    } finally {
      submitLock.current = false
    }
  }

  const error = submitError
    ? messageForError(submitError, 'passwordReset')
    : null
  function submitForm(event: React.FormEvent<HTMLFormElement>) {
    void handleSubmit(onSubmit)(event)
  }
  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-center px-4 py-8 sm:px-6">
      <div className="mb-6">
        <AppBrand name="CRM" logoSrc={null} />
      </div>
      <section className="space-y-6 rounded-xl border bg-card p-5 shadow-sm sm:p-8">
        {sent ? (
          <>
            <PageHeader title="Revisá tu email" titleRef={heading} />
            <p>
              Si {form.getValues('email')} tiene un usuario, en unos minutos te
              llega un enlace para crear una contraseña nueva (o tu invitación,
              si todavía no la aceptaste). El enlace vence en 1 hora. Revisá
              también el correo no deseado.
            </p>
            <p>
              Si ya lo pediste varias veces, esperá unos 20 minutos antes de
              pedirlo de nuevo.
            </p>
            <Link
              className="inline-flex min-h-11 items-center text-primary underline"
              to="/login"
            >
              Volver a Ingresar
            </Link>
            <button
              className="min-h-11 text-primary underline"
              type="button"
              onClick={() => setSent(false)}
            >
              ¿No llegó? Pedilo de nuevo
            </button>
          </>
        ) : (
          <>
            <PageHeader
              title="Recuperá tu contraseña"
              description="Te mandamos un enlace para crear una nueva."
              back={{ to: '/login', label: '← Volver' }}
            />
            <form noValidate className="space-y-5" onSubmit={submitForm}>
              <Field data-invalid={Boolean(errors.email)}>
                <FieldLabel htmlFor="reset-email">Email</FieldLabel>
                <Input
                  id="reset-email"
                  type="email"
                  className="h-11"
                  autoComplete="email"
                  readOnly={requestReset.isPending}
                  aria-invalid={Boolean(errors.email)}
                  aria-describedby={
                    errors.email ? 'reset-email-error' : undefined
                  }
                  {...register('email', {
                    onChange: () => {
                      clearErrors('email')
                      setSubmitError(null)
                    },
                  })}
                />
                {errors.email && (
                  <FieldError id="reset-email-error">
                    {fieldErrorText('email', errors.email)}
                  </FieldError>
                )}
              </Field>
              {error && (
                <FormAlert
                  variant="error"
                  title={error.title}
                  description={error.description}
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
                pending={requestReset.isPending || isSubmitting}
                pendingLabel="Enviando…"
                className="h-11 w-full"
              >
                Enviar enlace
              </SubmitButton>
            </form>
          </>
        )}
      </section>
    </main>
  )
}
