import { useImperativeHandle, useRef } from 'react'
import { MoreHorizontal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { Stage } from '@/api/types'
import { StageSwatch } from './StageColorOption'

export type StageAction = 'edit' | 'moveUp' | 'moveDown' | 'moveTo' | 'delete'

export interface StageListItemProps {
  stage: Stage
  isInitial: boolean
  typeLabel: string
  canManage: boolean
  isFirst: boolean
  isLast: boolean
  // The operation pending in THIS row.
  pending: 'update' | 'move' | 'delete' | null
  // BR-F14: a stage write is pending somewhere in the list.
  movesBlocked: boolean
  onAction: (action: StageAction, trigger: HTMLButtonElement) => void
  triggerRef?: React.Ref<HTMLButtonElement>
}

const pendingText = {
  update: 'Guardando…',
  move: 'Moviendo…',
  delete: 'Eliminando…',
} as const

// T-002-F202 (002/§14.2, BR-F2, BR-F4, BR-F12, BR-F14): one stage. Presentation only.
export function StageListItem({
  stage,
  isInitial,
  typeLabel,
  canManage,
  isFirst,
  isLast,
  pending,
  movesBlocked,
  onAction,
  triggerRef,
}: StageListItemProps) {
  const ownRef = useRef<HTMLButtonElement>(null)
  // The parent keeps the trigger by id to give the focus back (INV-F6); the item keeps its own
  // to tell onAction which button the menu came from.
  useImperativeHandle(triggerRef, () => ownRef.current as HTMLButtonElement)
  const details = [
    typeLabel,
    isInitial ? 'Etapa inicial' : null,
    pending ? pendingText[pending] : null,
  ]
    .filter(Boolean)
    .join(' · ')
  const busy = pending !== null
  const actions: Array<{
    action: StageAction
    label: string
    disabled: boolean
  }> = [
    { action: 'edit', label: 'Editar', disabled: busy },
    ...(isFirst
      ? []
      : [
          {
            action: 'moveUp' as const,
            label: 'Subir',
            disabled: busy || movesBlocked,
          },
        ]),
    ...(isLast
      ? []
      : [
          {
            action: 'moveDown' as const,
            label: 'Bajar',
            disabled: busy || movesBlocked,
          },
        ]),
    ...(isFirst && isLast
      ? []
      : [
          {
            action: 'moveTo' as const,
            label: 'Mover a otra posición…',
            disabled: busy || movesBlocked,
          },
        ]),
    { action: 'delete', label: 'Eliminar', disabled: busy },
  ]
  return (
    <li className="rounded-lg border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1 [overflow-wrap:anywhere]">
          <p className="flex items-center gap-2 font-semibold">
            <StageSwatch color={stage.color} />
            {stage.name}
          </p>
          <p className="text-sm text-muted-foreground">{details}</p>
        </div>
        {canManage && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              {/* Never disabled: the focus must not get lost (BR-F14). */}
              <Button
                ref={ownRef}
                type="button"
                variant="ghost"
                size="icon"
                aria-label={`Acciones para ${stage.name}`}
              >
                <MoreHorizontal aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {actions.map(({ action, label, disabled }) => (
                <DropdownMenuItem
                  key={action}
                  disabled={disabled}
                  onSelect={() => {
                    if (ownRef.current) onAction(action, ownRef.current)
                  }}
                >
                  {label}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    </li>
  )
}
