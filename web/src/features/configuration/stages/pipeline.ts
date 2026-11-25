import type { Stage, StageColor, StageType, StageUpdate } from '@/api/types'

// T-002-F201 (002/INV-F4, DD-F5): pure rules of the pipeline. They give the same result as
// validatePipeline and the list operations of internal/settings/pipeline.go (table of T-002-B301).
export type PipelineViolation = 'first_not_open' | 'no_won'

type StageLike = Pick<Stage, 'id' | 'type'>

/** The first stage is evaluated first: [lost] is first_not_open, not no_won. [] is no_won. */
export function pipelineViolation(
  stages: ReadonlyArray<Pick<Stage, 'type'>>,
): PipelineViolation | null {
  if (stages.length === 0) return 'no_won'
  if (stages[0]?.type !== 'open') return 'first_not_open'
  return stages.some((stage) => stage.type === 'won') ? null : 'no_won'
}

// The one place where a list is put back at positions 1..n: whoever has a `position` gets it renumbered.
function renumbered<T extends StageLike>(stages: readonly T[]): T[] {
  return stages.map((stage, index) =>
    'position' in stage ? { ...stage, position: index + 1 } : stage,
  )
}

function inserted<T>(stages: readonly T[], item: T, position: number): T[] {
  const index = Math.min(Math.max(position, 1), stages.length + 1) - 1
  return [...stages.slice(0, index), item, ...stages.slice(index)]
}

export function afterCreate<T extends StageLike>(
  stages: readonly T[],
  draft: T,
  position: number | 'end',
): T[] {
  return renumbered(
    inserted(stages, draft, position === 'end' ? stages.length + 1 : position),
  )
}

export function afterMove<T extends StageLike>(
  stages: readonly T[],
  stageId: string,
  position: number,
): T[] {
  const moving = stages.find((stage) => stage.id === stageId)
  if (!moving) return renumbered(stages)
  const rest = stages.filter((stage) => stage.id !== stageId)
  return renumbered(inserted(rest, moving, position))
}

export function afterDelete<T extends StageLike>(
  stages: readonly T[],
  stageId: string,
): T[] {
  return renumbered(stages.filter((stage) => stage.id !== stageId))
}

export function afterTypeChange<T extends StageLike>(
  stages: readonly T[],
  stageId: string,
  type: StageType,
): T[] {
  return renumbered(
    stages.map((stage) => (stage.id === stageId ? { ...stage, type } : stage)),
  )
}

// The order of the contract (StageColor in the OpenAPI): the suggestion is the first one not in use.
const colorOrder: readonly StageColor[] = [
  'gray',
  'blue',
  'teal',
  'green',
  'yellow',
  'orange',
  'red',
  'purple',
]

export function suggestStageColor(
  stages: ReadonlyArray<Pick<Stage, 'color'>>,
): StageColor {
  const used = new Set(stages.map((stage) => stage.color))
  return colorOrder.find((color) => !used.has(color)) ?? 'gray'
}

/** One option per position; "después de" names the stage that comes before in the list without the moved one. */
export function moveOptions(
  stages: ReadonlyArray<Pick<Stage, 'id' | 'name'>>,
  stageId: string,
): Array<{ position: number; label: string; current: boolean }> {
  const rest = stages.filter((stage) => stage.id !== stageId)
  const currentPosition = stages.findIndex((stage) => stage.id === stageId) + 1
  return stages.map((_, index) => {
    const position = index + 1
    const label =
      position === 1
        ? `${position} (al principio)`
        : position === stages.length
          ? `${position} (al final)`
          : `${position} (después de «${rest[position - 2]?.name ?? ''}»)`
    return { position, label, current: position === currentPosition }
  })
}

/** Only the keys that changed; {} if nothing did (002/INV-F9). Never a null; the name is sent trimmed. */
export function stageChanges(
  initial: Pick<Stage, 'name' | 'type' | 'color'>,
  values: Pick<Stage, 'name' | 'type' | 'color'>,
): StageUpdate | Record<string, never> {
  const changes: StageUpdate = {}
  const name = values.name.trim()
  if (name !== initial.name) changes.name = name
  if (values.type !== initial.type) changes.type = values.type
  if (values.color !== initial.color) changes.color = values.color
  return changes
}
