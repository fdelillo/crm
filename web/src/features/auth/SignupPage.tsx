import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link, useNavigate } from 'react-router'
import { AppBrand } from '@/components/AppBrand'
import { FormAlert } from '@/components/FormAlert'
import { PasswordInput } from '@/components/PasswordInput'
import { SubmitButton } from '@/components/SubmitButton'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  messageForError,
  applyServerFieldErrors,
  fieldErrorMessage,
} from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import { useIndustryTemplates, useSignup } from './api'
import { CurrencyPicker } from './components/CurrencyPicker'
import { IndustryTemplatePicker } from './components/IndustryTemplatePicker'
import { signupSchema, type SignupFormValues } from './schemas'

const signupFields = [
  'name',
  'email',
  'password',
  'company_name',
  'industry_template_code',
  'base_currency',
] as const

export function SignupPage() {
  const navigate = useNavigate()
  const templates = useIndustryTemplates()
  const signup = useSignup()
  const [submitError, setSubmitError] = useState<unknown>(null)
  const [serverFieldErrors, setServerFieldErrors] = useState<
    Partial<Record<(typeof signupFields)[number], string>>
  >({})
  const form = useForm<SignupFormValues>({
    resolver: zodResolver(signupSchema),
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    defaultValues: {
      name: '',
      email: '',
      password: '',
      company_name: '',
      industry_template_code: '',
      base_currency: 'ARS',
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    },
  })
  const {
    register,
    handleSubmit,
    setError,
    setFocus,
    control,
    formState: { errors },
  } = form

  async function onSubmit(values: SignupFormValues) {
    setSubmitError(null)
    setServerFieldErrors({})
    try {
      await signup.mutateAsync(signupSchema.parse(values))
    } catch (error) {
      if (isApiError(error) && error.code === 'validation_failed') {
        const unknownErrors = applyServerFieldErrors(
          setError,
          error.fieldErrors,
          signupFields,
        )
        const mappedErrors = Object.fromEntries(
          error.fieldErrors
            .filter((fieldError) =>
              signupFields.includes(
                fieldError.field as (typeof signupFields)[number],
              ),
            )
            .map((fieldError) => [
              fieldError.field,
              fieldErrorMessage(fieldError.field, fieldError.code),
            ]),
        ) as Partial<Record<(typeof signupFields)[number], string>>
        setServerFieldErrors(mappedErrors)
        const firstField = signupFields.find((field) => mappedErrors[field])
        if (firstField) setFocus(firstField)
        if (unknownErrors.length === 0) return
      }
      setSubmitError(error)
    }
  }

  function retrySubmit() {
    void handleSubmit(onSubmit)()
  }

  function recoverPassword() {
    const email = form.getValues('email').trim().toLowerCase()
    void navigate('/forgot-password', {
      state: { email },
    })
  }

  function useAnotherEmail() {
    setSubmitError(null)
    setFocus('email')
  }

  function clearServerFieldError(field: (typeof signupFields)[number]) {
    setServerFieldErrors((current) => {
      if (!current[field]) return current
      const next = { ...current }
      delete next[field]
      return next
    })
  }

  const message = submitError ? messageForError(submitError, 'signup') : null
  const existingEmail =
    isApiError(submitError) && submitError.code === 'email_already_registered'
  const errorId = (field: (typeof signupFields)[number]) =>
    errors[field] || serverFieldErrors[field] ? `${field}-error` : undefined

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-center px-4 py-8 sm:px-6">
      <div className="mb-6">
        <AppBrand name="CRM" logoSrc={null} />
      </div>
      <section className="space-y-6 rounded-xl border bg-card p-5 shadow-sm sm:p-8">
        <header className="space-y-2">
          <h1 tabIndex={-1}>Creá tu cuenta</h1>
          <p className="text-muted-foreground">
            Registrá tu empresa y empezá a usar el sistema.
          </p>
        </header>

        <form
          className="space-y-6"
          noValidate
          onSubmit={handleSubmit(onSubmit)}
        >
          <FieldSet className="gap-4">
            <legend className="text-base font-medium">Tus datos</legend>
            <Field
              data-invalid={Boolean(errors.name || serverFieldErrors.name)}
            >
              <FieldLabel htmlFor="name">Tu nombre</FieldLabel>
              <Input
                id="name"
                autoComplete="name"
                readOnly={signup.isPending}
                aria-invalid={Boolean(errors.name || serverFieldErrors.name)}
                aria-describedby={errorId('name')}
                {...register('name', {
                  onChange: () => clearServerFieldError('name'),
                })}
              />
              {(errors.name || serverFieldErrors.name) && (
                <FieldError id={errorId('name')}>
                  {serverFieldErrors.name ??
                    fieldErrorMessage('name', errors.name?.message as never)}
                </FieldError>
              )}
            </Field>
            <Field
              data-invalid={Boolean(errors.email || serverFieldErrors.email)}
            >
              <FieldLabel htmlFor="email">Email</FieldLabel>
              <Input
                id="email"
                type="email"
                autoComplete="email"
                readOnly={signup.isPending}
                aria-invalid={Boolean(errors.email || serverFieldErrors.email)}
                aria-describedby={errorId('email')}
                {...register('email', {
                  onChange: () => clearServerFieldError('email'),
                })}
              />
              {(errors.email || serverFieldErrors.email) && (
                <FieldError id={errorId('email')}>
                  {serverFieldErrors.email ??
                    fieldErrorMessage('email', errors.email?.message as never)}
                </FieldError>
              )}
            </Field>
            <Field
              data-invalid={Boolean(
                errors.password || serverFieldErrors.password,
              )}
            >
              <FieldLabel htmlFor="password">Contraseña</FieldLabel>
              <PasswordInput
                id="password"
                autoComplete="new-password"
                readOnly={signup.isPending}
                aria-invalid={Boolean(
                  errors.password || serverFieldErrors.password,
                )}
                aria-describedby={errorId('password') ?? 'password-help'}
                {...register('password', {
                  onChange: () => clearServerFieldError('password'),
                })}
              />
              <FieldDescription id="password-help">
                Mínimo 10 caracteres.
              </FieldDescription>
              {(errors.password || serverFieldErrors.password) && (
                <FieldError id={errorId('password')}>
                  {serverFieldErrors.password ??
                    fieldErrorMessage(
                      'password',
                      errors.password?.message as never,
                    )}
                </FieldError>
              )}
            </Field>
          </FieldSet>

          <FieldSet className="gap-4">
            <legend className="text-base font-medium">Tu empresa</legend>
            <Field
              data-invalid={Boolean(
                errors.company_name || serverFieldErrors.company_name,
              )}
            >
              <FieldLabel htmlFor="company_name">
                Nombre de la empresa
              </FieldLabel>
              <Input
                id="company_name"
                autoComplete="organization"
                readOnly={signup.isPending}
                aria-invalid={Boolean(
                  errors.company_name || serverFieldErrors.company_name,
                )}
                aria-describedby={errorId('company_name')}
                {...register('company_name', {
                  onChange: () => clearServerFieldError('company_name'),
                })}
              />
              {(errors.company_name || serverFieldErrors.company_name) && (
                <FieldError id={errorId('company_name')}>
                  {serverFieldErrors.company_name ??
                    fieldErrorMessage(
                      'company_name',
                      errors.company_name?.message as never,
                    )}
                </FieldError>
              )}
            </Field>
            <Field
              data-invalid={Boolean(
                errors.industry_template_code ||
                serverFieldErrors.industry_template_code,
              )}
            >
              <FieldLabel>Rubro de tu empresa</FieldLabel>
              <Controller
                control={control}
                name="industry_template_code"
                render={({ field }) => (
                  <IndustryTemplatePicker
                    templates={templates.data?.items}
                    status={
                      templates.isPending
                        ? 'pending'
                        : templates.isError
                          ? 'error'
                          : 'success'
                    }
                    onRetry={() => void templates.refetch()}
                    value={field.value}
                    onChange={(value) => {
                      clearServerFieldError('industry_template_code')
                      field.onChange(value)
                    }}
                    error={
                      serverFieldErrors.industry_template_code ??
                      (errors.industry_template_code
                        ? fieldErrorMessage(
                            'industry_template_code',
                            errors.industry_template_code.message as never,
                          )
                        : undefined)
                    }
                    name={field.name}
                  />
                )}
              />
            </Field>
            <Field
              data-invalid={Boolean(
                errors.base_currency || serverFieldErrors.base_currency,
              )}
            >
              <FieldLabel>Moneda base</FieldLabel>
              <Controller
                control={control}
                name="base_currency"
                render={({ field }) => (
                  <CurrencyPicker
                    value={field.value}
                    onChange={(value) => {
                      clearServerFieldError('base_currency')
                      field.onChange(value)
                    }}
                    name={field.name}
                    error={
                      serverFieldErrors.base_currency ??
                      (errors.base_currency
                        ? fieldErrorMessage(
                            'base_currency',
                            errors.base_currency.message as never,
                          )
                        : undefined)
                    }
                  />
                )}
              />
            </Field>
          </FieldSet>

          {message && (
            <FormAlert
              variant="error"
              title={message.title}
              description={message.description}
              requestId={message.requestId}
              action={
                existingEmail
                  ? { label: 'Recuperar contraseña', onClick: recoverPassword }
                  : message.retryable
                    ? { label: 'Reintentar', onClick: retrySubmit }
                    : undefined
              }
              secondaryAction={
                existingEmail
                  ? { label: 'Usar otro email', onClick: useAnotherEmail }
                  : undefined
              }
            />
          )}

          <SubmitButton
            pending={signup.isPending}
            disabled={!templates.isSuccess || templates.data.items.length === 0}
            pendingLabel="Creando cuenta…"
          >
            Crear cuenta
          </SubmitButton>
        </form>

        <p className="text-center text-sm">
          ¿Ya tenés cuenta?{' '}
          <Link
            className="text-primary underline underline-offset-4"
            to="/login"
          >
            Ingresá
          </Link>
        </p>
      </section>
    </main>
  )
}
