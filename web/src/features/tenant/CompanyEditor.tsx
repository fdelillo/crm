import { useEffect, useMemo, useRef, useState } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { Controller, useForm } from 'react-hook-form'
import { toast } from 'sonner'
import {
  applyServerFieldErrors,
  fieldErrorText,
  messageForError,
} from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import type { Tenant } from '@/api/types'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { FormAlert } from '@/components/FormAlert'
import { PageHeader } from '@/components/PageHeader'
import { SubmitButton } from '@/components/SubmitButton'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useUnsavedChangesGuard } from '@/lib/unsavedChanges'
import { logoUrl, useUpdateTenant } from './api'
import {
  LOGO_INPUT_ID,
  LOGO_REMOVE_ID,
  LogoUploader,
} from './components/LogoUploader'
import {
  buildTenantUpdate,
  companyFields,
  companySchema,
  tenantToFormValues,
  type CompanyFormValues,
} from './schemas'
import { timeZoneGroups } from './timezones'
import { useLogoActions } from './useLogoActions'

const currencyNames = { ARS: 'Pesos (ARS)', USD: 'Dólares (USD)' } as const
type TextField = Exclude<keyof CompanyFormValues, 'timezone'>
const textFields: {
  name: TextField
  label: string
  help?: string
  autoComplete?: string
  type?: string
  inputMode?: 'tel' | 'email' | 'numeric'
}[] = [
  { name: 'name', label: 'Nombre de la empresa', autoComplete: 'organization' },
  { name: 'legal_name', label: 'Razón social' },
  {
    name: 'tax_id',
    label: 'CUIT',
    help: '11 números, con o sin guiones.',
    inputMode: 'numeric',
  },
  { name: 'address', label: 'Dirección', autoComplete: 'street-address' },
  { name: 'phone', label: 'Teléfono', autoComplete: 'tel', inputMode: 'tel' },
  {
    name: 'email',
    label: 'Email de contacto',
    type: 'email',
    autoComplete: 'email',
    inputMode: 'email',
  },
]

// T-F602, T-F603 (US-4, BR-F09, BR-F10, INV-F15, INV-F18, DD-F32, ui.md §13.11).
export function CompanyEditor({ tenant }: { tenant: Tenant }) {
  const update = useUpdateTenant()
  const logo = useLogoActions()
  const [submitError, setSubmitError] = useState<unknown>(null)
  const submitLock = useRef(false)
  const form = useForm<CompanyFormValues>({
    resolver: zodResolver(companySchema),
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    defaultValues: tenantToFormValues(tenant),
  })
  const {
    register,
    control,
    handleSubmit,
    reset,
    setError,
    setFocus,
    clearErrors,
    formState: { errors, isDirty, dirtyFields },
  } = form

  // DD-F32: a single guard per screen. A form with changes, or a logo being prepared or
  // uploaded, asks before leaving; an expired session never does.
  const blocker = useUnsavedChangesGuard(isDirty || logo.pending !== null)

  const zones = useMemo(
    () => timeZoneGroups(tenant.timezone),
    [tenant.timezone],
  )

  const wasConfirming = useRef(false)
  useEffect(() => {
    // The dialog has no trigger element: put the focus back by hand (INV-F10).
    if (wasConfirming.current && !logo.confirmingRemoval) {
      const remove = document.getElementById(LOGO_REMOVE_ID)
      const target =
        remove instanceof HTMLButtonElement && !remove.disabled
          ? remove
          : document.getElementById(LOGO_INPUT_ID)
      target?.focus()
    }
    wasConfirming.current = logo.confirmingRemoval
  }, [logo.confirmingRemoval, logo.pending])

  async function onSubmit(values: CompanyFormValues) {
    if (submitLock.current) return
    const patch = buildTenantUpdate(values, dirtyFields)
    if (Object.keys(patch).length === 0) return
    submitLock.current = true
    setSubmitError(null)
    try {
      const saved = await update.mutateAsync(patch)
      reset(tenantToFormValues(saved))
      toast('Guardamos los datos de la empresa.')
    } catch (error) {
      if (isApiError(error) && error.code === 'validation_failed') {
        const unknown = applyServerFieldErrors(
          setError,
          error.fieldErrors,
          companyFields,
        )
        const first = companyFields.find((field) =>
          error.fieldErrors.some((item) => item.field === field),
        )
        if (first) setFocus(first)
        if (error.fieldErrors.length > 0 && unknown.length === 0) return
      }
      // A 401 goes to the login through the global handler (§12.4).
      if (!(isApiError(error) && error.code === 'unauthenticated'))
        setSubmitError(error)
    } finally {
      submitLock.current = false
    }
  }

  const message = submitError ? messageForError(submitError, 'tenant') : null
  const saving = update.isPending
  const src = logoUrl(tenant)
  return (
    <>
      <PageHeader
        title="Datos de la empresa"
        description="Se usan en los PDF de tus presupuestos."
        back={{ to: '/settings', label: '← Ajustes' }}
      />
      <div className="space-y-8">
        <LogoUploader
          companyName={tenant.name}
          logoSrc={src}
          pending={logo.pending}
          error={logo.error}
          onSelectFile={logo.selectFile}
          onRemove={logo.askToRemove}
          onRetry={logo.retry}
        />
        <form
          aria-labelledby="company-data-heading"
          className="space-y-5"
          noValidate
          onSubmit={handleSubmit(onSubmit)}
        >
          <h2 id="company-data-heading" className="text-lg font-semibold">
            Datos
          </h2>
          {textFields.map(
            ({ name, label, help, type, autoComplete, inputMode }) => {
              const errorId = errors[name] ? `company-${name}-error` : undefined
              const helpId = help ? `company-${name}-help` : undefined
              return (
                <Field key={name} data-invalid={Boolean(errors[name])}>
                  <FieldLabel htmlFor={`company-${name}`}>
                    {label}
                    {name === 'name' && <span aria-hidden="true"> *</span>}
                  </FieldLabel>
                  <Input
                    id={`company-${name}`}
                    type={type ?? 'text'}
                    className="h-11"
                    autoComplete={autoComplete}
                    inputMode={inputMode}
                    readOnly={saving}
                    aria-required={name === 'name' ? true : undefined}
                    aria-invalid={Boolean(errors[name])}
                    aria-describedby={
                      [helpId, errorId].filter(Boolean).join(' ') || undefined
                    }
                    {...register(name, {
                      onChange: () => {
                        clearErrors(name)
                        setSubmitError(null)
                      },
                    })}
                  />
                  {help && (
                    <FieldDescription id={helpId}>{help}</FieldDescription>
                  )}
                  {errors[name] && (
                    <FieldError id={errorId}>
                      {fieldErrorText(name, errors[name])}
                    </FieldError>
                  )}
                </Field>
              )
            },
          )}
          <Field data-invalid={Boolean(errors.timezone)}>
            <FieldLabel htmlFor="company-timezone">Zona horaria</FieldLabel>
            <Controller
              control={control}
              name="timezone"
              render={({ field }) => (
                <Select
                  value={field.value}
                  disabled={saving}
                  onValueChange={(value) => {
                    field.onChange(value)
                    clearErrors('timezone')
                    setSubmitError(null)
                  }}
                >
                  <SelectTrigger
                    id="company-timezone"
                    ref={field.ref}
                    className="h-11 w-full"
                    onBlur={field.onBlur}
                    aria-invalid={Boolean(errors.timezone)}
                    aria-describedby={
                      errors.timezone ? 'company-timezone-error' : undefined
                    }
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className="max-h-72">
                    {zones.argentina.length > 0 && (
                      <SelectGroup>
                        <SelectLabel>Argentina</SelectLabel>
                        {zones.argentina.map((zone) => (
                          <SelectItem key={zone} value={zone}>
                            {zone}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    )}
                    <SelectGroup>
                      <SelectLabel>Otras zonas</SelectLabel>
                      {zones.others.map((zone) => (
                        <SelectItem key={zone} value={zone}>
                          {zone}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              )}
            />
            {errors.timezone && (
              <FieldError id="company-timezone-error">
                {fieldErrorText('timezone', errors.timezone)}
              </FieldError>
            )}
          </Field>
          <div>
            <p>Moneda base: {currencyNames[tenant.base_currency]}</p>
            <p className="text-sm text-muted-foreground">
              No se puede cambiar.
            </p>
          </div>
          {message && (
            <FormAlert
              variant="error"
              title={message.title}
              description={message.description}
              requestId={message.requestId}
              action={
                message.retryable
                  ? {
                      label: 'Reintentar',
                      onClick: () => void handleSubmit(onSubmit)(),
                    }
                  : undefined
              }
            />
          )}
          <SubmitButton
            pending={saving}
            pendingLabel="Guardando…"
            disabled={!isDirty}
            className="h-11"
          >
            Guardar cambios
          </SubmitButton>
        </form>
      </div>
      <ConfirmDialog
        open={logo.confirmingRemoval}
        onOpenChange={(open) => {
          if (!open && logo.pending !== 'remove') logo.cancelRemoval()
        }}
        title="¿Quitar el logo?"
        description="Vamos a mostrar las iniciales de tu empresa en su lugar. Podés subir otro logo cuando quieras."
        confirmLabel="Quitar"
        pendingLabel="Quitando logo…"
        destructive
        pending={logo.pending === 'remove'}
        onConfirm={logo.confirmRemoval}
      />
      <ConfirmDialog
        open={blocker.state === 'blocked'}
        onOpenChange={(open) => {
          if (!open && blocker.state === 'blocked') blocker.reset()
        }}
        title="Tenés cambios sin guardar. ¿Salir igual?"
        description="Si salís ahora, vas a perder lo que cargaste."
        confirmLabel="Salir"
        pendingLabel="Saliendo…"
        cancelLabel="Quedarme"
        pending={false}
        onConfirm={() => {
          if (blocker.state === 'blocked') blocker.proceed()
        }}
      />
    </>
  )
}
