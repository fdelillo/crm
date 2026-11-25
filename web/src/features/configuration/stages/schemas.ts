import { z } from 'zod'
import { fieldErrorMessage } from '@/api/errorMessages'
import type { Stage, StageColor, StageCreate, StageType } from '@/api/types'
import { countCodePoints } from '@/lib/textLength'
import { trimSpace } from '@/lib/trimSpace'
import {
  afterCreate,
  afterTypeChange,
  pipelineViolation,
  suggestStageColor,
} from './pipeline'
import type { StageTexts } from './texts'

// T-002-F202 (002/§14.2 C-03, §17.1, INV-F10, INV-F11): the form of a stage. `before` is the stage
// the new one goes in front of ('end' for the last place), not a number: a number chosen over a list
// that a pending move then changes would point at another stage (INV-F11 computes the position when
// it is sent, over the list the last confirmed write left).
export interface StageFormValues {
  name: string
  type: StageType
  color: StageColor
  before: string
}
export const END = 'end'

export const stageTypes: readonly StageType[] = ['open', 'won', 'lost']
// The order of the color picker (ui.md §14.2); the suggestion follows the order of pipeline.ts.
export const stageColorChoices: readonly StageColor[] = [
  'blue',
  'teal',
  'green',
  'yellow',
  'orange',
  'red',
  'purple',
  'gray',
]

export function positionOf(
  stages: readonly Stage[],
  before: string,
): number | 'end' {
  const index = stages.findIndex((stage) => stage.id === before)
  return before === END || index === -1 ? 'end' : index + 1
}

/** "Antes de «{primera etapa de cierre}»" if there is one; if not, "Al final". */
export function suggestedBefore(stages: readonly Stage[]): string {
  return stages.find((stage) => stage.type !== 'open')?.id ?? END
}

export function newStageValues(stages: readonly Stage[]): StageFormValues {
  return {
    name: '',
    type: 'open',
    color: suggestStageColor(stages),
    before: suggestedBefore(stages),
  }
}

export interface StageSchemaContext {
  mode: 'create' | 'edit'
  stages: readonly Stage[]
  stageId?: string
  texts: Pick<StageTexts, 'firstNotOpen' | 'noWon'>
}

// The name is compared and counted trimmed with trimSpace, the trim of the server (never .trim()).
export function stageFormSchema({
  mode,
  stages,
  stageId,
  texts,
}: StageSchemaContext) {
  const taken = (name: string) => {
    const wanted = name.toLowerCase()
    return stages.some(
      (stage) => stage.id !== stageId && stage.name.toLowerCase() === wanted,
    )
  }
  return z
    .object({
      name: z.string().superRefine((value, context) => {
        const name = trimSpace(value)
        const message =
          name === ''
            ? fieldErrorMessage('name', 'required', 'stage')
            : countCodePoints(name) > 60
              ? fieldErrorMessage('name', 'too_long', 'stage')
              : taken(name)
                ? 'Ya hay una etapa con ese nombre.'
                : null
        if (message) context.addIssue({ code: 'custom', message })
      }),
      type: z.enum(stageTypes),
      color: z.enum(stageColorChoices),
      before: z.string(),
    })
    .superRefine((values, context) => {
      const initial = stages.find((stage) => stage.id === stageId)
      if (mode === 'edit' && initial?.type === values.type) return
      const result =
        mode === 'create'
          ? afterCreate<Pick<Stage, 'id' | 'type'>>(
              stages,
              { id: '', type: values.type },
              positionOf(stages, values.before),
            )
          : afterTypeChange(stages, stageId ?? '', values.type)
      const violation = pipelineViolation(result)
      if (violation)
        context.addIssue({
          code: 'custom',
          path: ['type'],
          message: violation === 'no_won' ? texts.noWon : texts.firstNotOpen,
        })
    }) satisfies z.ZodType<StageFormValues>
}

/** The body of POST /stages: the position is the one of the list that is current NOW (INV-F11). */
export function toStageCreate(
  values: StageFormValues,
  stages: readonly Stage[],
): StageCreate {
  const position = positionOf(stages, values.before)
  return {
    name: trimSpace(values.name),
    type: values.type,
    color: values.color,
    ...(position === 'end' ? {} : { position }),
  }
}
