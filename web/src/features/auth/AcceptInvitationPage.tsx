import { useEffect, useRef, useState } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { Link } from 'react-router'
import { isApiError } from '@/api/errors'
import {
  applyServerFieldErrors,
  fieldErrorText,
  messageForError,
} from '@/api/errorMessages'
import { FormAlert } from '@/components/FormAlert'
import { PageHeader } from '@/components/PageHeader'
import { PasswordInput } from '@/components/PasswordInput'
import { Skeleton } from '@/components/ui/skeleton'
import { SubmitButton } from '@/components/SubmitButton'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { formatDateTime } from '@/lib/format'
import { useLinkToken } from './useLinkToken'
import { useAcceptInvitation, useInvitationPreview } from './api'
import { useSession } from './session'
import {
  invitationAcceptSchema,
  type InvitationAcceptFormValues,
} from './schemas'

export function AcceptInvitationPage() {
  const token = useLinkToken()
  const session = useSession()
  const previewRequest = useInvitationPreview(token)
  const loadPreview = previewRequest.mutate
  const acceptInvitation = useAcceptInvitation()
  const heading = useRef<HTMLHeadingElement>(null)
  const submitLock = useRef(false)
  const previewStarted = useRef(false)
  const [submitError, setSubmitError] = useState<unknown>(null)
  const preview = previewRequest.data
  const form = useForm<InvitationAcceptFormValues>({
    resolver: zodResolver(invitationAcceptSchema(preview?.email ?? '')),
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    defaultValues: { token: token ?? '', name: '', password: '' },
  })
  const {
    register,
    handleSubmit,
    setFocus,
    setError,
    clearErrors,
    formState: { errors },
  } = form

  useEffect(() => {
    if (token && !previewStarted.current) {
      previewStarted.current = true
      loadPreview()
    }
  }, [token, loadPreview])

  const invalidToken =
    (isApiError(previewRequest.error) &&
      previewRequest.error.code === 'token_invalid') ||
    (isApiError(submitError) && submitError.code === 'token_invalid')

  useEffect(() => {
    if (invalidToken) heading.current?.focus()
  }, [invalidToken])

  async function onSubmit(values: InvitationAcceptFormValues) {
    if (!token || !preview || submitLock.current) return
    submitLock.current = true
    setSubmitError(null)
    clearErrors()
    try {
      await acceptInvitation.mutateAsync(
        invitationAcceptSchema(preview.email).parse({ ...values, token }),
      )
    } catch (error) {
      if (isApiError(error) && error.code === 'validation_failed') {
        const fields = ['name', 'password'] as const
        const unknownErrors = applyServerFieldErrors(
          setError,
          error.fieldErrors,
          fields,
        )
        const firstField = fields.find((field) =>
          error.fieldErrors.some((item) => item.field === field),
        )
        if (firstField) setFocus(firstField)
        if (error.fieldErrors.length > 0 && unknownErrors.length === 0) return
      }
      setSubmitError(error)
    } finally {
      submitLock.current = false
    }
  }

  const previewMessage = previewRequest.error
    ? messageForError(previewRequest.error, 'invitationPreview')
    : null
  const submitMessage = submitError
    ? messageForError(submitError, 'invitationAccept')
    : null
  const error = invalidToken ? null : submitMessage

  return (
    <main className="mx-auto w-full max-w-xl p-4 pb-8">
      <PageHeader title="Aceptar invitación" titleRef={heading} />
      {!token ? (
        <div className="space-y-4">
          <FormAlert
            variant="warning"
            announce={false}
            title="Este enlace está incompleto."
            description="Abrilo de nuevo desde el email."
          />
        </div>
      ) : invalidToken ? (
        <div className="space-y-4">
          <FormAlert
            variant="warning"
            announce={false}
            title="Esta invitación ya no es válida."
            description="Venció, ya se usó o la reemplazó una más nueva."
          />
          <p>
            Si todavía no la aceptaste,{' '}
            <Link className="text-primary underline" to="/forgot-password">
              pedí una nueva con tu email
            </Link>
          </p>
          <div className="flex flex-wrap gap-3">
            <Link
              className="inline-flex min-h-11 items-center text-primary underline"
              to="/login"
            >
              ¿Ya tenés usuario? Ingresá
            </Link>
          </div>
        </div>
      ) : previewRequest.isPending || previewRequest.isIdle ? (
        <section
          aria-label="Cargando invitación"
          aria-busy="true"
          className="space-y-4"
        >
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-80 w-full" />
        </section>
      ) : previewRequest.isError ? (
        <div className="space-y-4">
          <FormAlert
            variant="error"
            title={previewMessage?.title ?? 'No pudimos cargar la invitación.'}
            description={previewMessage?.description}
            requestId={previewMessage?.requestId}
            action={
              previewMessage?.retryable ||
              (isApiError(previewRequest.error) &&
                previewRequest.error.status === 429)
                ? {
                    label: 'Reintentar',
                    onClick: () => loadPreview(),
                  }
                : undefined
            }
          />
        </div>
      ) : preview ? (
        <div className="space-y-5">
          <section
            aria-label="Datos de la invitación"
            className="space-y-2 rounded-lg border p-4"
          >
            <h2 className="text-lg font-semibold">
              Te invitaron a {preview.tenant_name} como{' '}
              {preview.role === 'admin' ? 'Administrador' : 'Operador'}
            </h2>
            <p>
              La invitación vence el{' '}
              {formatDateTime(
                preview.expires_at,
                Intl.DateTimeFormat().resolvedOptions().timeZone,
              )}
              .
            </p>
          </section>
          {session.data?.user && (
            <FormAlert
              variant="warning"
              announce={false}
              title={`Tenés una sesión abierta como ${session.data.user.email}. Si aceptás, se va a cerrar.`}
            />
          )}
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
          <form
            className="space-y-5"
            noValidate
            onSubmit={handleSubmit(onSubmit)}
          >
            <Field>
              <FieldLabel htmlFor="invitation-email">Email</FieldLabel>
              <Input
                id="invitation-email"
                className="h-11"
                type="email"
                autoComplete="email"
                readOnly
                value={preview.email}
              />
              <FieldDescription>
                Este es el email al que se envió la invitación.
              </FieldDescription>
            </Field>
            <Field data-invalid={Boolean(errors.name)}>
              <FieldLabel htmlFor="invitation-name">Tu nombre</FieldLabel>
              <Input
                id="invitation-name"
                className="h-11"
                autoComplete="name"
                aria-invalid={Boolean(errors.name)}
                aria-describedby={
                  errors.name ? 'invitation-name-error' : undefined
                }
                {...register('name', { onChange: () => clearErrors('name') })}
              />
              {errors.name && (
                <FieldError id="invitation-name-error">
                  {fieldErrorText('name', errors.name)}
                </FieldError>
              )}
            </Field>
            <Field data-invalid={Boolean(errors.password)}>
              <FieldLabel htmlFor="invitation-password">Contraseña</FieldLabel>
              <PasswordInput
                id="invitation-password"
                className="h-11"
                autoComplete="new-password"
                aria-invalid={Boolean(errors.password)}
                aria-describedby={
                  errors.password ? 'invitation-password-error' : undefined
                }
                {...register('password', {
                  onChange: () => clearErrors('password'),
                })}
              />
              <FieldDescription>
                Usá entre 10 y 128 caracteres.
              </FieldDescription>
              {errors.password && (
                <FieldError id="invitation-password-error">
                  {fieldErrorText('password', errors.password)}
                </FieldError>
              )}
            </Field>
            <SubmitButton
              pending={acceptInvitation.isPending}
              pendingLabel="Aceptando…"
              className="h-11"
            >
              Aceptar invitación
            </SubmitButton>
          </form>
        </div>
      ) : null}
    </main>
  )
}
