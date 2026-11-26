import { Controller, useForm } from 'react-hook-form'
import type { Stage } from '@/api/types'
import { FormDialog } from '@/components/FormDialog'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { afterMove, moveOptions, pipelineViolation } from '../pipeline'

export interface StageMoveDialogProps {
  stage: Stage
  stages: Stage[]
  firstNotOpenText: string
  returnFocusRef: React.RefObject<HTMLElement | null>
  onClose: () => void
  onMove: (position: number) => void
}

// T-002-F202 (002/§14.2 C-03 "Mover a otra posición", BR-F3): the chosen position goes out only if
// it is not the current one and the pipeline stays valid; if not, the dialog closes or says why.
export function StageMoveDialog({
  stage,
  stages,
  firstNotOpenText,
  returnFocusRef,
  onClose,
  onMove,
}: StageMoveDialogProps) {
  const options = moveOptions(stages, stage.id)
  const current = options.find((option) => option.current)?.position ?? 1
  const form = useForm<{ position: string }>({
    defaultValues: { position: String(current) },
  })
  const error = form.formState.errors.position
  return (
    <FormDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
      title={`Mover «${stage.name}»`}
      submitLabel="Mover"
      pendingLabel="Moviendo…"
      pending={false}
      onSubmit={form.handleSubmit(({ position: chosen }) => {
        const position = Number(chosen)
        if (position === current) return onClose()
        if (pipelineViolation(afterMove(stages, stage.id, position)))
          return form.setError('position', {
            type: 'validate',
            message: firstNotOpenText,
          })
        onMove(position)
      })}
      returnFocusRef={returnFocusRef}
    >
      <Field data-invalid={Boolean(error)}>
        <FieldLabel htmlFor="stage-position">Posición</FieldLabel>
        <Controller
          control={form.control}
          name="position"
          render={({ field }) => (
            <Select
              value={field.value}
              onValueChange={(value) => {
                field.onChange(value)
                form.clearErrors('position')
              }}
            >
              <SelectTrigger
                id="stage-position"
                className="h-11 w-full"
                aria-invalid={Boolean(error)}
                aria-describedby={error ? 'stage-position-error' : undefined}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {options.map((option) => (
                  <SelectItem
                    key={option.position}
                    value={String(option.position)}
                  >
                    {option.current
                      ? `${option.label} (lugar actual)`
                      : option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        {error && (
          <FieldError id="stage-position-error">{error.message}</FieldError>
        )}
      </Field>
    </FormDialog>
  )
}
