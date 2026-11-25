import type { StageColor, StageType } from '@/api/types'

// T-002-F201 (002/ui.md §16.3, §19.2): a Record per contract enum, so a new value does not compile
// without its text.
export const stageTypeLabels: Record<StageType, string> = {
  open: 'Abierta',
  won: 'Cierre ganado',
  lost: 'Cierre perdido',
}

export const stageColorNames: Record<StageColor, string> = {
  gray: 'Gris',
  blue: 'Azul',
  teal: 'Verde azulado',
  green: 'Verde',
  yellow: 'Amarillo',
  orange: 'Naranja',
  red: 'Rojo',
  purple: 'Violeta',
}
