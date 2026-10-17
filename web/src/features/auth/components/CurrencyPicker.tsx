import { FieldError } from '@/components/ui/field'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import type { Currency } from '@/api/types'

export interface CurrencyPickerProps {
  value: Currency
  onChange: (value: Currency) => void
  error?: string
  name: string
}

export function CurrencyPicker({
  value,
  onChange,
  error,
  name,
}: CurrencyPickerProps) {
  const errorId = error ? `${name}-error` : undefined
  return (
    <div className="space-y-2">
      <RadioGroup
        aria-label="Moneda base"
        aria-invalid={Boolean(error)}
        aria-describedby={errorId}
        name={name}
        value={value}
        onValueChange={(next) => onChange(next as Currency)}
        className="grid grid-cols-2 gap-3"
      >
        {[
          { code: 'ARS', label: 'Pesos (ARS)' },
          { code: 'USD', label: 'Dólares (USD)' },
        ].map(({ code, label }) => (
          <div
            key={code}
            className="flex min-h-11 items-center gap-3 rounded-md border px-3 py-2"
          >
            <RadioGroupItem id={`${name}-${code}`} value={code} />
            <label
              htmlFor={`${name}-${code}`}
              className="flex-1 cursor-pointer"
            >
              {label}
            </label>
          </div>
        ))}
      </RadioGroup>
      <p className="text-sm text-muted-foreground">
        La usamos para totales y reportes. No se puede cambiar después.
      </p>
      {error && <FieldError id={errorId}>{error}</FieldError>}
    </div>
  )
}
