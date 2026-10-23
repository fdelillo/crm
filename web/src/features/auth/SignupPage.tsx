import { useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link, useNavigate } from 'react-router'
import { AppBrand } from '@/components/AppBrand'
import { FormAlert } from '@/components/FormAlert'
import { PageHeader } from '@/components/PageHeader'
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
  fieldErrorText,
} from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import { queryKeys } from '@/api/queryKeys'
import { useIndustryTemplates, useSignup } from './api'
import { CurrencyPicker } from './components/CurrencyPicker'
import { IndustryTemplatePicker } from './components/IndustryTemplatePicker'
import { signupSchema, type SignupFormValues } from './signupSchema'

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
  const queryClient = useQueryClient()
  const submitLock = useRef(false)
  const templates = useIndustryTemplates()
  const signup = useSignup()
  const [submitError, setSubmitError] = useState<unknown>(null)
  const form = useForm<SignupFormValues>({
    resolver: zodResolver(signupSchema),
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    defaultValues: {
      name: '',
      email: '',
      password: '',
      company_name: '',
      base_currency: 'ARS',
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    },
  })
  const {
    register,
    handleSubmit,
    setError,
    clearErrors,
    setFocus,
    control,
    formState: { errors },
  } = form

  async function onSubmit(values: SignupFormValues) {
    if (submitLock.current) return
    submitLock.current = true
    setSubmitError(null)
    try {
      await signup.mutateAsync(signupSchema.parse(values))
    } catch (error) {
      if (isApiError(error) && error.code === 'validation_failed') {
        if (
          error.fieldErrors.some(
            (fieldError) =>
              fieldError.field === 'industry_template_code' &&
              fieldError.code === 'unknown_template',
          )
        ) {
          void queryClient.invalidateQueries({
            queryKey: queryKeys.industryTemplates,
          })
        }
        const unknownErrors = applyServerFieldErrors(
          setError,
          error.fieldErrors,
          signupFields,
        )
        const firstField = signupFields.find((field) =>
          error.fieldErrors.some((fieldError) => fieldError.field === field),
        )
        if (firstField) setFocus(firstField)
        if (error.fieldErrors.length > 0 && unknownErrors.length === 0) return
      }
      setSubmitError(error)
    } finally {
      submitLock.current = false
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

  const message = submitError ? messageForError(submitError, 'signup') : null
  const existingEmail =
    isApiError(submitError) && submitError.code === 'email_already_registered'
  const canRecoverPassword =
    isApiError(submitError) && submitError.suggestedAction === 'password_reset'
  const errorId = (field: (typeof signupFields)[number]) =>
    errors[field] ? `${field}-error` : undefined

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-center px-4 py-8 sm:px-6">
      <div className="mb-6">
        <AppBrand name="CRM" logoSrc={null} />
      </div>
      <section className="space-y-6 rounded-xl border bg-card p-5 shadow-sm sm:p-8">
        <PageHeader
          title="Creá tu cuenta"
          description="Registrá tu empresa y empezá a usar el sistema."
        />

        <form
          className="space-y-6"
          noValidate
          onSubmit={handleSubmit(onSubmit)}
        >
          <FieldSet className="gap-4">
            <legend className="text-base font-medium">Tus datos</legend>
            <Field data-invalid={Boolean(errors.name)}>
              <FieldLabel htmlFor="name">Tu nombre</FieldLabel>
              <Input
                id="name"
                className="h-11"
                autoComplete="name"
                readOnly={signup.isPending}
                aria-invalid={Boolean(errors.name)}
                aria-describedby={errorId('name')}
                {...register('name', {
                  onChange: () => clearErrors('name'),
                })}
              />
              {errors.name && (
                <FieldError id={errorId('name')}>
                  {fieldErrorText('name', errors.name)}
                </FieldError>
              )}
            </Field>
            <Field data-invalid={Boolean(errors.email)}>
              <FieldLabel htmlFor="email">Email</FieldLabel>
              <Input
                id="email"
                type="email"
                className="h-11"
                autoComplete="email"
                readOnly={signup.isPending}
                aria-invalid={Boolean(errors.email)}
                aria-describedby={errorId('email')}
                {...register('email', {
                  onChange: () => clearErrors('email'),
                })}
              />
              {errors.email && (
                <FieldError id={errorId('email')}>
                  {fieldErrorText('email', errors.email)}
                </FieldError>
              )}
            </Field>
            <Field data-invalid={Boolean(errors.password)}>
              <FieldLabel htmlFor="password">Contraseña</FieldLabel>
              <PasswordInput
                id="password"
                className="h-11"
                autoComplete="new-password"
                readOnly={signup.isPending}
                aria-invalid={Boolean(errors.password)}
                aria-describedby={
                  ['password-help', errorId('password')]
                    .filter(Boolean)
                    .join(' ') || undefined
                }
                {...register('password', {
                  onChange: () => clearErrors('password'),
                })}
              />
              <FieldDescription id="password-help">
                Mínimo 10 caracteres.
              </FieldDescription>
              {errors.password && (
                <FieldError id={errorId('password')}>
                  {fieldErrorText('password', errors.password)}
                </FieldError>
              )}
            </Field>
          </FieldSet>

          <FieldSet className="gap-4">
            <legend className="text-base font-medium">Tu empresa</legend>
            <Field data-invalid={Boolean(errors.company_name)}>
              <FieldLabel htmlFor="company_name">
                Nombre de la empresa
              </FieldLabel>
              <Input
                id="company_name"
                className="h-11"
                autoComplete="organization"
                readOnly={signup.isPending}
                aria-invalid={Boolean(errors.company_name)}
                aria-describedby={errorId('company_name')}
                {...register('company_name', {
                  onChange: () => clearErrors('company_name'),
                })}
              />
              {errors.company_name && (
                <FieldError id={errorId('company_name')}>
                  {fieldErrorText('company_name', errors.company_name)}
                </FieldError>
              )}
            </Field>
            <Field data-invalid={Boolean(errors.industry_template_code)}>
              <FieldLabel id="industry-template-heading">
                Rubro de tu empresa
              </FieldLabel>
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
                    ref={field.ref}
                    labelledBy="industry-template-heading"
                    onChange={(value) => {
                      clearErrors('industry_template_code')
                      field.onChange(value)
                    }}
                    error={fieldErrorText(
                      'industry_template_code',
                      errors.industry_template_code,
                    )}
                    disabled={signup.isPending}
                    name={field.name}
                  />
                )}
              />
            </Field>
            <Field data-invalid={Boolean(errors.base_currency)}>
              <FieldLabel id="base-currency-heading">Moneda base</FieldLabel>
              <Controller
                control={control}
                name="base_currency"
                render={({ field }) => (
                  <CurrencyPicker
                    value={field.value}
                    ref={field.ref}
                    labelledBy="base-currency-heading"
                    onChange={(value) => {
                      clearErrors('base_currency')
                      field.onChange(value)
                    }}
                    name={field.name}
                    disabled={signup.isPending}
                    error={fieldErrorText(
                      'base_currency',
                      errors.base_currency,
                    )}
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
                canRecoverPassword
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
            className="h-11 w-full sm:w-auto"
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
