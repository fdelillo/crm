import { Controller, type UseFormReturn } from 'react-hook-form'
import type { Stage, StageType } from '@/api/types'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { stageTypeLabels } from '../labels'
import {
  END,
  stageColorChoices,
  stageTypes,
  type StageFormValues,
} from '../schemas'
import { StageColorOption } from './StageColorOption'

export interface StageFormProps {
  mode: 'create' | 'edit'
  stages: Stage[]
  typeDescriptions: Record<StageType, string>
  form: UseFormReturn<StageFormValues>
}

const optionCard =
  'flex min-h-11 cursor-pointer items-start gap-3 rounded-md border p-3'

// T-002-F202 (002/§14.2 C-03, §16.2, §18): the fields of "Nueva etapa" and "Editar etapa".
export function StageForm({
  mode,
  stages,
  typeDescriptions,
  form,
}: StageFormProps) {
  const {
    register,
    control,
    clearErrors,
    formState: { errors },
  } = form
  const nameError = errors.name ? 'stage-name-error' : undefined
  const typeError = errors.type ? 'stage-type-error' : undefined
  return (
    <>
      <Field data-invalid={Boolean(errors.name)}>
        <FieldLabel htmlFor="stage-name">Nombre</FieldLabel>
        <Input
          id="stage-name"
          autoComplete="off"
          className="h-11"
          aria-invalid={Boolean(errors.name)}
          aria-describedby={nameError}
          {...register('name', { onChange: () => clearErrors('name') })}
        />
        {errors.name && (
          <FieldError id={nameError}>{errors.name.message}</FieldError>
        )}
      </Field>
      <Field data-invalid={Boolean(errors.type)}>
        <FieldLabel id="stage-type-label">Tipo</FieldLabel>
        <Controller
          control={control}
          name="type"
          render={({ field }) => (
            <RadioGroup
              aria-labelledby="stage-type-label"
              aria-invalid={Boolean(errors.type)}
              aria-describedby={typeError}
              value={field.value}
              onValueChange={(value) => {
                field.onChange(value)
                clearErrors('type')
              }}
            >
              {stageTypes.map((type) => (
                <label
                  key={type}
                  htmlFor={`stage-type-${type}`}
                  className={optionCard}
                >
                  <RadioGroupItem
                    id={`stage-type-${type}`}
                    value={type}
                    aria-labelledby={`stage-type-${type}-label`}
                    aria-describedby={`stage-type-${type}-help`}
                  />
                  <span>
                    <span
                      id={`stage-type-${type}-label`}
                      className="font-medium"
                    >
                      {stageTypeLabels[type]}
                    </span>
                    <FieldDescription id={`stage-type-${type}-help`}>
                      {typeDescriptions[type]}
                    </FieldDescription>
                  </span>
                </label>
              ))}
            </RadioGroup>
          )}
        />
        {errors.type && (
          <FieldError id={typeError}>{errors.type.message}</FieldError>
        )}
      </Field>
      <Field>
        <FieldLabel id="stage-color-label">Color</FieldLabel>
        <Controller
          control={control}
          name="color"
          render={({ field }) => (
            <RadioGroup
              aria-labelledby="stage-color-label"
              className="grid-cols-2"
              value={field.value}
              onValueChange={field.onChange}
            >
              {stageColorChoices.map((color) => (
                <label
                  key={color}
                  htmlFor={`stage-color-${color}`}
                  className={`${optionCard} items-center`}
                >
                  <RadioGroupItem id={`stage-color-${color}`} value={color} />
                  <StageColorOption color={color} />
                </label>
              ))}
            </RadioGroup>
          )}
        />
      </Field>
      {mode === 'create' && (
        <Field>
          <FieldLabel htmlFor="stage-before">Ubicación</FieldLabel>
          <Controller
            control={control}
            name="before"
            render={({ field }) => (
              <Select value={field.value} onValueChange={field.onChange}>
                <SelectTrigger id="stage-before" className="h-11 w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={END}>Al final</SelectItem>
                  {stages.map((stage) => (
                    <SelectItem key={stage.id} value={stage.id}>
                      {`Antes de «${stage.name}»`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </Field>
      )}
    </>
  )
}
