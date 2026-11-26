import { cn } from 'cn'
import type { StageColor } from '@/api/types'
import { stageColorNames } from '../labels'

// Whole class names, so Tailwind finds them. The tokens are --stage-* (ui.md §19.2).
const swatchClasses: Record<StageColor, string> = {
  gray: 'bg-stage-gray',
  blue: 'bg-stage-blue',
  teal: 'bg-stage-teal',
  green: 'bg-stage-green',
  yellow: 'bg-stage-yellow',
  orange: 'bg-stage-orange',
  red: 'bg-stage-red',
  purple: 'bg-stage-purple',
}

/** The color sample is decorative: the name of the color is what carries the information (BR-F2). */
export function StageSwatch({ color }: { color: StageColor }) {
  return (
    <span
      aria-hidden="true"
      className={cn('size-4 shrink-0 rounded-full', swatchClasses[color])}
    />
  )
}

export interface StageColorOptionProps {
  color: StageColor
}

// T-002-F202 (002/§16.2): sample (aria-hidden) and the name of the color in text.
export function StageColorOption({ color }: StageColorOptionProps) {
  return (
    <span className="flex items-center gap-2">
      <StageSwatch color={color} />
      <span>{stageColorNames[color]}</span>
    </span>
  )
}
