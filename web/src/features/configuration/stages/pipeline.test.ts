import { describe, expect, it } from 'vitest'

import type { Stage, StageColor, StageType } from '@/api/types'
import { fourStages, genericStages } from '@/test/configuration'

import { stageColorNames, stageTypeLabels } from './labels'
import {
  afterCreate,
  afterDelete,
  afterMove,
  afterTypeChange,
  moveOptions,
  pipelineViolation,
  stageChanges,
  suggestStageColor,
} from './pipeline'

// T-002-F201 (002/INV-F4): the same table as T-002-B301 (internal/settings/pipeline_test.go).
const o: StageType = 'open'
const w: StageType = 'won'
const l: StageType = 'lost'
const typed = (types: StageType[]) => types.map((type) => ({ type }))

function byName(stages: Stage[], name: string): Stage {
  const stage = stages.find((s) => s.name === name)
  if (!stage) throw new Error(`No stage ${name}`)
  return stage
}
const names = (stages: Stage[]) => stages.map((s) => s.name)

describe('pipelineViolation', () => {
  it.each([[[]], [[o]], [[o, l]]] as StageType[][][])(
    'reports no_won for %j',
    (types) => {
      expect(pipelineViolation(typed(types))).toBe('no_won')
    },
  )
  // The first stage is checked first: [w, o] and [l] also lack a won stage or start closed.
  it.each([[[w, o]], [[l, o, w]], [[l]]] as StageType[][][])(
    'reports first_not_open for %j',
    (types) => {
      expect(pipelineViolation(typed(types))).toBe('first_not_open')
    },
  )
  it.each([
    [[o, w]],
    [[o, w, l]],
    [[o, l, w]],
    [[o, o, w, w]],
  ] as StageType[][][])('accepts %j', (types) => {
    expect(pipelineViolation(typed(types))).toBeNull()
  })
})

describe('afterDelete', () => {
  it('leaves no won stage when deleting Finalizado', () => {
    const stages = genericStages()
    const result = afterDelete(stages, byName(stages, 'Finalizado').id)
    expect(names(result)).toEqual([
      'Presupuesto',
      'Aprobado',
      'En curso',
      'Perdido',
    ])
    expect(pipelineViolation(result)).toBe('no_won')
  })
  it('is valid when deleting Aprobado, with the others in order and dense positions', () => {
    const stages = genericStages()
    const result = afterDelete(stages, byName(stages, 'Aprobado').id)
    expect(names(result)).toEqual([
      'Presupuesto',
      'En curso',
      'Finalizado',
      'Perdido',
    ])
    expect(result.map((s) => s.position)).toEqual([1, 2, 3, 4])
    expect(pipelineViolation(result)).toBeNull()
  })
  it('is valid when deleting Presupuesto, whose successor is open (Aprobado is the second one)', () => {
    const stages = genericStages()
    expect(
      pipelineViolation(afterDelete(stages, byName(stages, 'Presupuesto').id)),
    ).toBeNull()
  })
  it('makes first_not_open when deleting the first of [o, w, l]', () => {
    const stages = genericStages().filter(
      (s) => s.name !== 'Aprobado' && s.name !== 'En curso',
    )
    const result = afterDelete(stages, stages[0]?.id ?? '')
    expect(pipelineViolation(result)).toBe('first_not_open')
  })
  it('does not modify the input', () => {
    const stages = genericStages()
    afterDelete(stages, stages[0]?.id ?? '')
    expect(names(stages)).toHaveLength(5)
  })
})

describe('afterMove', () => {
  it('first_not_open when Perdido goes to position 1', () => {
    const stages = genericStages()
    const result = afterMove(stages, byName(stages, 'Perdido').id, 1)
    expect(pipelineViolation(result)).toBe('first_not_open')
  })
  it('is valid when En curso goes from 3 to 1', () => {
    const stages = genericStages()
    const result = afterMove(stages, byName(stages, 'En curso').id, 1)
    expect(names(result)).toEqual([
      'En curso',
      'Presupuesto',
      'Aprobado',
      'Finalizado',
      'Perdido',
    ])
    expect(result.map((s) => s.position)).toEqual([1, 2, 3, 4, 5])
    expect(pipelineViolation(result)).toBeNull()
  })
  it('is valid when Finalizado goes to position 5', () => {
    const stages = genericStages()
    const result = afterMove(stages, byName(stages, 'Finalizado').id, 5)
    expect(names(result)).toEqual([
      'Presupuesto',
      'Aprobado',
      'En curso',
      'Perdido',
      'Finalizado',
    ])
    expect(pipelineViolation(result)).toBeNull()
  })
  it('moves down in the middle', () => {
    const stages = genericStages()
    const result = afterMove(stages, byName(stages, 'Aprobado').id, 3)
    expect(names(result)).toEqual([
      'Presupuesto',
      'En curso',
      'Aprobado',
      'Finalizado',
      'Perdido',
    ])
  })
  it('keeps the order when the position does not change', () => {
    const stages = genericStages()
    expect(names(afterMove(stages, byName(stages, 'En curso').id, 3))).toEqual(
      names(stages),
    )
  })
  it('does not modify the input', () => {
    const stages = genericStages()
    afterMove(stages, byName(stages, 'En curso').id, 1)
    expect(names(stages)[0]).toBe('Presupuesto')
    expect(stages.map((s) => s.position)).toEqual([1, 2, 3, 4, 5])
  })
})

describe('afterTypeChange', () => {
  it('no_won when Finalizado becomes open', () => {
    const stages = genericStages()
    const result = afterTypeChange(stages, byName(stages, 'Finalizado').id, o)
    expect(pipelineViolation(result)).toBe('no_won')
  })
  it('first_not_open when Presupuesto becomes won', () => {
    const stages = genericStages()
    const result = afterTypeChange(stages, byName(stages, 'Presupuesto').id, w)
    expect(pipelineViolation(result)).toBe('first_not_open')
  })
  it('is valid when En curso becomes won, and nothing else changes', () => {
    const stages = genericStages()
    const result = afterTypeChange(stages, byName(stages, 'En curso').id, w)
    expect(pipelineViolation(result)).toBeNull()
    expect(result.map((s) => s.type)).toEqual([o, o, w, w, l])
    expect(names(result)).toEqual(names(stages))
    expect(stages.map((s) => s.type)).toEqual([o, o, o, w, l])
  })
})

describe('afterCreate', () => {
  const draft = buildDraft()
  it('first_not_open for a won stage at position 1', () => {
    const result = afterCreate(genericStages(), { ...buildDraft(), type: w }, 1)
    expect(pipelineViolation(result)).toBe('first_not_open')
  })
  it('is valid for an open stage at the end', () => {
    const stages = genericStages()
    const result = afterCreate(stages, draft, 'end')
    expect(result).toHaveLength(6)
    expect(result[5]?.id).toBe('draft')
    expect(pipelineViolation(result)).toBeNull()
  })
  it('is valid for an open stage at position 4, which ends up fourth', () => {
    const result = afterCreate(genericStages(), draft, 4)
    expect(result.map((s) => s.id).indexOf('draft')).toBe(3)
    expect(result).toHaveLength(6)
    expect(pipelineViolation(result)).toBeNull()
  })
  it('gives dense positions to the stages that have them', () => {
    const result = afterCreate(genericStages(), draft, 2)
    expect(result.map((s) => s.position)).toEqual([1, 2, 3, 4, 5, 6])
  })
})

function buildDraft(): Stage {
  return { id: 'draft', name: 'Nueva', type: o, color: 'red', position: 6 }
}

describe('moveOptions', () => {
  it('lists every position for En curso, with the current one marked', () => {
    const stages = genericStages()
    const options = moveOptions(stages, byName(stages, 'En curso').id)
    expect(options).toHaveLength(5)
    expect(options.map((option) => option.position)).toEqual([1, 2, 3, 4, 5])
    expect(options.map((option) => option.current)).toEqual([
      false,
      false,
      true,
      false,
      false,
    ])
    expect(options.map((option) => option.label)).toEqual([
      '1 (al principio)',
      '2 (después de «Presupuesto»)',
      '3 (después de «Aprobado»)',
      '4 (después de «Finalizado»)',
      '5 (al final)',
    ])
  })
  it('names the stage that would come before, never the moved one', () => {
    const stages = genericStages()
    const options = moveOptions(stages, byName(stages, 'Presupuesto').id)
    expect(options.map((option) => option.label)).toEqual([
      '1 (al principio)',
      '2 (después de «Aprobado»)',
      '3 (después de «En curso»)',
      '4 (después de «Finalizado»)',
      '5 (al final)',
    ])
    expect(options[0]?.current).toBe(true)
  })
})

describe('suggestStageColor', () => {
  it('suggests the first free color in the contract order', () => {
    const colors = genericStages().map((s) => ({ color: s.color }))
    expect(suggestStageColor(colors)).toBe('orange')
  })
  it('falls back to gray when the eight are used', () => {
    const all = Object.keys(stageColorNames) as StageColor[]
    expect(suggestStageColor(all.map((color) => ({ color })))).toBe('gray')
  })
  it('starts with gray on an empty list', () => {
    expect(suggestStageColor([])).toBe('gray')
  })
})

describe('stageChanges', () => {
  const initial = { name: 'En curso', type: o, color: 'yellow' } as const
  it('is empty when nothing changed', () => {
    expect(stageChanges(initial, { ...initial })).toEqual({})
  })
  it('sends only the name, without the spaces at the ends', () => {
    const changes = stageChanges(initial, { ...initial, name: '  Obra  ' })
    expect(changes).toStrictEqual({ name: 'Obra' })
  })
  it('does not count the surrounding spaces as a change', () => {
    expect(
      stageChanges(initial, { ...initial, name: '  En curso ' }),
    ).toStrictEqual({})
  })
  // T-002-F201 (002/INV-F9; revisión 1 de Codex del PR #78): trimmed like the server (Go's TrimSpace).
  it('trims U+0085 like the server, so a name made of it is empty', () => {
    expect(stageChanges(initial, { ...initial, name: '\u0085' })).toStrictEqual(
      { name: '' },
    )
  })
  it('keeps a U+FEFF the server keeps, so only the color changes', () => {
    const withMark = { ...initial, name: '\ufeffEn curso' }
    expect(stageChanges(withMark, { ...withMark, color: 'red' })).toStrictEqual(
      { color: 'red' },
    )
  })
  it('sends name and color, never null', () => {
    const changes = stageChanges(initial, {
      ...initial,
      name: 'Obra',
      color: 'red',
    })
    expect(changes).toStrictEqual({ name: 'Obra', color: 'red' })
    expect(Object.values(changes)).not.toContain(null)
  })
  it('sends only the type when only the type changed', () => {
    expect(stageChanges(initial, { ...initial, type: w })).toStrictEqual({
      type: 'won',
    })
  })
})

describe('labels', () => {
  it('has a text for every stage type', () => {
    expect(stageTypeLabels).toEqual({
      open: 'Abierta',
      won: 'Cierre ganado',
      lost: 'Cierre perdido',
    })
  })
  it('has a name for every stage color', () => {
    expect(stageColorNames).toEqual({
      gray: 'Gris',
      blue: 'Azul',
      teal: 'Verde azulado',
      green: 'Verde',
      yellow: 'Amarillo',
      orange: 'Naranja',
      red: 'Rojo',
      purple: 'Violeta',
    })
  })
})

describe('fourStages fixture', () => {
  it('is valid and has no lost stage', () => {
    expect(pipelineViolation(fourStages())).toBeNull()
    expect(fourStages().some((s) => s.type === 'lost')).toBe(false)
  })
})
