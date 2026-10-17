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
}: IndustryTemplatePickerProps) {
  const errorId = error ? `${name}-error` : undefined
  if (status === 'pending') {
    return (
      <div
        role="status"
        aria-label="Cargando rubros"
        aria-busy="true"
        className="space-y-3"
      >
        <Skeleton className="h-11 w-full" />
        <Skeleton className="h-11 w-full" />
      </div>
    )
  }
  if (status === 'error' || !templates?.length) {
    return (
      <div className="space-y-2">
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
      <RadioGroup
        aria-label="Rubro de tu empresa"
        orientation="vertical"
        aria-invalid={Boolean(error)}
        aria-describedby={errorId}
        name={name}
        disabled={disabled}
        value={value}
        onValueChange={onChange}
        onKeyDown={(event) => {
          if (disabled) return
          if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
          const currentCode = (event.target as HTMLElement)
            .closest('[role="radio"]')
            ?.getAttribute('value')
          const currentIndex = templates.findIndex(
            (template) => template.code === currentCode,
          )
          if (currentIndex < 0) return
          const delta = event.key === 'ArrowDown' ? 1 : -1
          const nextIndex =
            (currentIndex + delta + templates.length) % templates.length
          const nextTemplate = templates[nextIndex]
          if (nextTemplate) onChange(nextTemplate.code)
        }}
        className="gap-2"
      >
        {templates.map((template, index) => {
          const itemId = `${name}-${index}`
          return (
            <div
              key={template.code}
              className="flex min-h-11 items-center gap-3 rounded-md border px-3 py-2"
            >
              <RadioGroupItem id={itemId} value={template.code} />
              <label htmlFor={itemId} className="flex-1 cursor-pointer">
                {template.name}
              </label>
            </div>
          )
        })}
      </RadioGroup>
      <FieldDescription>
        Precargamos la configuración típica de tu rubro.
      </FieldDescription>
      {error && <FieldError id={errorId}>{error}</FieldError>}
    </div>
  )
}
