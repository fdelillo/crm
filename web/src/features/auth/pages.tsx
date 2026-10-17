import { useRef, useState } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm, useWatch } from 'react-hook-form'
import { Link, useLocation, useNavigate } from 'react-router'
import { AppBrand } from '@/components/AppBrand'
import { FormAlert } from '@/components/FormAlert'
import { LockedNotice } from '@/components/LockedNotice'
import { PageHeader } from '@/components/PageHeader'
import { PasswordInput } from '@/components/PasswordInput'
import { SubmitButton } from '@/components/SubmitButton'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  applyServerFieldErrors,
  fieldErrorText,
  messageForError,
} from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import type { FieldErrorCode } from '@/api/types'
import { useLogin } from './api'
import { loginSchema, type LoginFormValues } from './schemas'

const loginFields = ['email', 'password'] as const

export function LoginPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const login = useLogin()
  const submitLock = useRef(false)
  const [submitError, setSubmitError] = useState<unknown>(null)
  const [lockedError, setLockedError] = useState<unknown>(null)
  const entryState = location.state as {
    email?: string
    unsavedChangesLost?: boolean
  } | null
  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    defaultValues: { email: entryState?.email ?? '', password: '' },
  })
  const {
    register,
    handleSubmit,
    clearErrors,
    setError,
    setFocus,
    getValues,
    formState: { errors, isSubmitting },
  } = form
  const emailValue = useWatch({ control: form.control, name: 'email' })
  const reason = new URLSearchParams(location.search).get('reason')
  const expired = reason === 'session_expired'
  const notice =
    reason === 'password_changed'
      ? 'Listo, cambiaste tu contraseña. Ingresá con la nueva.'
      : reason === 'logged_out'
        ? 'Cerraste sesión.'
        : null

  async function onSubmit(values: LoginFormValues) {
    if (submitLock.current) return
    submitLock.current = true
    setSubmitError(null)
    setLockedError(null)
    try {
      await login.mutateAsync(loginSchema.parse(values))
    } catch (error) {
      if (isApiError(error) && error.code === 'validation_failed') {
        const unknownErrors = applyServerFieldErrors(
          setError,
          error.fieldErrors,
          loginFields,
        )
        const firstField = loginFields.find((field) =>
          error.fieldErrors.some((fieldError) => fieldError.field === field),
        )
        if (firstField) setFocus(firstField)
        if (error.fieldErrors.length > 0 && unknownErrors.length === 0) return
      }
      if (isApiError(error) && error.code === 'invalid_credentials') {
        form.setValue('password', '')
        setFocus('password')
      }
      if (isApiError(error) && error.code === 'login_locked') {
        setLockedError(error)
      } else {
        setSubmitError(error)
      }
    } finally {
      submitLock.current = false
    }
  }

  function retrySubmit() {
    void handleSubmit(onSubmit)()
  }

  function recoverPassword() {
    const email = getValues('email').trim().toLowerCase()
    void navigate('/forgot-password', { state: { email } })
  }

  const message = submitError ? messageForError(submitError, 'login') : null
  const lockedMessage = lockedError
    ? messageForError(lockedError, 'login')
    : null
  const errorId = (field: (typeof loginFields)[number]) =>
    errors[field] ? `${field}-error` : undefined

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-center px-4 py-8 sm:px-6">
      <div className="mb-6">
        <AppBrand name="CRM" logoSrc={null} />
      </div>
      <section className="space-y-6 rounded-xl border bg-card p-5 shadow-sm sm:p-8">
        <PageHeader title="Ingresá a tu cuenta" />
        {expired && (
          <div role="status" className="space-y-2 rounded-lg border p-4">
            <p>Tu sesión se cerró. Ingresá de nuevo para seguir.</p>
            <p>
              Las sesiones se cierran solas después de 24 horas sin uso o a los
              7 días.
            </p>
            {entryState?.unsavedChangesLost && (
              <p>Los cambios que no habías guardado se perdieron.</p>
            )}
          </div>
        )}
        {notice && (
          <div role="status" className="rounded-lg border p-4">
            <p>{notice}</p>
          </div>
        )}

        {lockedMessage ? (
          <LockedNotice
            title={lockedMessage.title}
            description={lockedMessage.description ?? ''}
            onResetPassword={recoverPassword}
            onRetry={() => setLockedError(null)}
          />
        ) : (
          <form
            noValidate
            className="space-y-5"
            onSubmit={handleSubmit(onSubmit)}
          >
            <Field data-invalid={Boolean(errors.email)}>
              <FieldLabel htmlFor="login-email">Email</FieldLabel>
              <Input
                id="login-email"
                type="email"
                className="h-11"
                autoComplete="email"
                readOnly={login.isPending}
                aria-invalid={Boolean(errors.email)}
                aria-describedby={errorId('email')}
                {...register('email', {
                  onChange: () => {
                    clearErrors('email')
                    setSubmitError(null)
                  },
                })}
              />
              {errors.email && (
                <FieldError id={errorId('email')}>
                  {fieldErrorText(
                    'email',
                    errors.email,
                    (errors.email.message ?? 'required') as FieldErrorCode,
                  )}
                </FieldError>
              )}
            </Field>
            <Field data-invalid={Boolean(errors.password)}>
              <FieldLabel htmlFor="login-password">Contraseña</FieldLabel>
              <PasswordInput
                id="login-password"
                className="h-11"
                autoComplete="current-password"
                readOnly={login.isPending}
                aria-invalid={Boolean(errors.password)}
                aria-describedby={errorId('password')}
                {...register('password', {
                  onChange: () => {
                    clearErrors('password')
                    setSubmitError(null)
                  },
                })}
              />
              {errors.password && (
                <FieldError id={errorId('password')}>
                  {fieldErrorText(
                    'password',
                    errors.password,
                    (errors.password.message ?? 'required') as FieldErrorCode,
                  )}
                </FieldError>
              )}
            </Field>

            <p className="text-sm">
              <Link
                to="/forgot-password"
                state={{ email: emailValue.trim().toLowerCase() }}
                className="min-h-11 text-primary underline underline-offset-4"
              >
                ¿Olvidaste tu contraseña?
              </Link>
            </p>

            {message && (
              <FormAlert
                variant="error"
                title={message.title}
                description={message.description}
                requestId={message.requestId}
                action={
                  message.retryable
                    ? { label: 'Reintentar', onClick: retrySubmit }
                    : undefined
                }
              />
            )}

            <SubmitButton
              pending={login.isPending || isSubmitting}
              pendingLabel="Ingresando…"
              className="h-11 w-full"
            >
              Ingresar
            </SubmitButton>
          </form>
        )}

        <p className="text-center text-sm">
          ¿No tenés cuenta?{' '}
          <Link
            className="text-primary underline underline-offset-4"
            to="/signup"
          >
            Registrá tu empresa
          </Link>
        </p>
      </section>
    </main>
  )
}
export { SignupPage } from './SignupPage'
export function ForgotPasswordPage() {
  return <h1>Olvidé mi contraseña</h1>
}
export function ResetPasswordPage() {
  return <h1>Restablecer contraseña</h1>
}
export function VerifyEmailPage() {
  return <h1>Confirmá tu email</h1>
}
export function AcceptInvitationPage() {
  return <h1>Aceptar invitación</h1>
}
