import type { Ref } from 'react'
import { FieldError } from '@/components/ui/field'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import type { Currency } from '@/api/types'

export interface CurrencyPickerProps {
  value: Currency
  onChange: (value: Currency) => void
  error?: string
  name: string
  disabled?: boolean
  ref?: Ref<HTMLButtonElement>
  labelledBy?: string
}

export function CurrencyPicker({
  value,
  onChange,
  error,
  name,
  disabled = false,
  ref,
  labelledBy,
}: CurrencyPickerProps) {
  const errorId = error ? `${name}-error` : undefined
  const helpId = `${name}-help`
  const describedBy = [helpId, errorId].filter(Boolean).join(' ')
  return (
    <div className="space-y-2">
      <RadioGroup
        aria-labelledby={labelledBy}
        aria-label={labelledBy ? undefined : 'Moneda base'}
        aria-invalid={Boolean(error)}
        aria-describedby={describedBy}
        name={name}
        disabled={disabled}
        value={value}
        onValueChange={(next) => onChange(next as Currency)}
        className="grid grid-cols-2 gap-3"
      >
        {[
          { code: 'ARS', label: 'Pesos (ARS)' },
          { code: 'USD', label: 'Dólares (USD)' },
        ].map(({ code, label }) => (
          <label
            key={code}
            htmlFor={`${name}-${code}`}
            className="flex min-h-11 cursor-pointer items-center gap-3 rounded-md border px-3 py-2"
          >
            <RadioGroupItem
              ref={code === value ? ref : undefined}
              id={`${name}-${code}`}
              value={code}
            />
            <span className="flex-1">{label}</span>
          </label>
        ))}
      </RadioGroup>
      <p id={helpId} className="text-sm text-muted-foreground">
        La usamos para totales y reportes. No se puede cambiar después.
      </p>
      {error && <FieldError id={errorId}>{error}</FieldError>}
    </div>
  )
}
