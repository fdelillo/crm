import type { Stage, StageColor, StageType } from '@/api/types'

// T-002-F201 (002/ui.md §16.4): fixtures of the configuration screens, with made-up data.
const stageIdPrefix = '0199bada-0000-7000-8000-0000000002'

export function buildStage(overrides: Partial<Stage> = {}): Stage {
  return {
    id: `${stageIdPrefix}00`,
    name: 'Etapa de prueba',
    type: 'open',
    color: 'blue',
    position: 1,
    ...overrides,
  }
}

function stageList(
  rows: ReadonlyArray<readonly [string, StageType, StageColor]>,
): Stage[] {
  return rows.map(([name, type, color], index) =>
    buildStage({
      id: `${stageIdPrefix}${String(index + 1).padStart(2, '0')}`,
      name,
      type,
      color,
      position: index + 1,
    }),
  )
}

/** The pipeline of the "Genérico" company: Presupuesto, Aprobado, En curso, Finalizado, Perdido. */
export function genericStages(): Stage[] {
  return stageList([
    ['Presupuesto', 'open', 'blue'],
    ['Aprobado', 'open', 'teal'],
    ['En curso', 'open', 'yellow'],
    ['Finalizado', 'won', 'green'],
    ['Perdido', 'lost', 'gray'],
  ])
}

/** The "cuatro etapas" variant, without Perdido. */
export function fourStages(): Stage[] {
  return genericStages().slice(0, 4)
}
