import type { Ref } from 'react'
import { Button } from '@/components/ui/button'
import { FieldDescription, FieldError } from '@/components/ui/field'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Skeleton } from '@/components/ui/skeleton'
import type { IndustryTemplate } from '@/api/types'

export interface IndustryTemplatePickerProps {
  templates: IndustryTemplate[] | undefined
  status: 'pending' | 'error' | 'success'
  onRetry: () => void
  value: string | undefined
  onChange: (code: string) => void
  error?: string
  name: string
  disabled?: boolean
  ref?: Ref<HTMLButtonElement>
  labelledBy?: string
}

export function IndustryTemplatePicker({
  templates,
  status,
  onRetry,
  value,
  onChange,
  error,
  name,
  disabled = false,
  ref,
  labelledBy,
}: IndustryTemplatePickerProps) {
  const errorId = error ? `${name}-error` : undefined
  const helpId = `${name}-help`
  const describedBy = [helpId, errorId].filter(Boolean).join(' ')
  const hasSelectedTemplate = templates?.some(
    (template) => template.code === value,
  )
  const help = (
    <FieldDescription id={helpId}>
      Precargamos la configuración típica de tu rubro.
    </FieldDescription>
  )
  if (status === 'pending') {
    return (
      <div className="space-y-2">
        {help}
        <div
          role="status"
          aria-label="Cargando rubros"
          aria-busy="true"
          className="space-y-3"
        >
          <Skeleton className="h-11 w-full" />
          <Skeleton className="h-11 w-full" />
        </div>
      </div>
    )
  }
  if (status === 'error' || !templates?.length) {
    return (
      <div className="space-y-2">
        {help}
        <p>No pudimos cargar los rubros.</p>
        <Button
          type="button"
          variant="outline"
          className="h-11"
          disabled={disabled}
          onClick={onRetry}
        >
          Reintentar
        </Button>
      </div>
    )
  }
  return (
    <div className="space-y-2">
      {help}
      {/* Radix handles radio arrow keys; E2E arrow-key coverage belongs to T-F702. */}
      <RadioGroup
        aria-labelledby={labelledBy}
        aria-label={labelledBy ? undefined : 'Rubro de tu empresa'}
        orientation="vertical"
        aria-invalid={Boolean(error)}
        aria-describedby={describedBy}
        name={name}
        disabled={disabled}
        value={value ?? ''}
        onValueChange={onChange}
        className="gap-2"
      >
        {templates.map((template, index) => {
          const itemId = `${name}-${index}`
          const receivesRef =
            template.code === value || (!hasSelectedTemplate && index === 0)
          return (
            <label
              key={template.code}
              htmlFor={itemId}
              className="flex min-h-11 cursor-pointer items-center gap-3 rounded-md border px-3 py-2"
            >
              <RadioGroupItem
                ref={receivesRef ? ref : undefined}
                id={itemId}
                value={template.code}
              />
              <span className="flex-1">{template.name}</span>
            </label>
          )
        })}
      </RadioGroup>
      {error && <FieldError id={errorId}>{error}</FieldError>}
    </div>
  )
}
