import { useRef } from 'react'
import { MoreHorizontal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { User } from '@/api/types'
import {
  userActionLabels,
  type UserAction,
  type UserActionAvailability,
} from '../rules'
import { UserStatusBadge } from './UserStatusBadge'

export interface UserListItemProps {
  user: User
  isCurrentUser: boolean
  actions: UserActionAvailability[]
  timeZone: string
  pendingActions: UserAction[]
  onAction: (action: UserAction, trigger: HTMLButtonElement) => void
}

export function UserListItem({
  user,
  isCurrentUser,
  actions,
  timeZone,
  pendingActions,
  onAction,
}: UserListItemProps) {
  const triggerRef = useRef<HTMLButtonElement>(null)
  const displayName = user.name ?? user.email
  return (
    <li className="list-none rounded-lg border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1 [overflow-wrap:anywhere]">
          <h2 className="font-semibold">
            {displayName}
            {isCurrentUser ? ' (vos)' : ''}
          </h2>
          <p>{user.email}</p>
          <p className="text-sm">
            {user.role === 'admin' ? 'Administrador' : 'Operador'}{' '}
            <UserStatusBadge
              status={user.status}
              invitationExpiresAt={user.invitation_expires_at}
              timeZone={timeZone}
            />
          </p>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              ref={triggerRef}
              type="button"
              variant="ghost"
              size="icon"
              aria-label={`Acciones para ${displayName}`}
            >
              <MoreHorizontal aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {actions.map((item) => (
              <DropdownMenuItem
                key={item.action}
                disabled={!item.enabled || pendingActions.includes(item.action)}
                onSelect={() => {
                  if (triggerRef.current)
                    onAction(item.action, triggerRef.current)
                }}
              >
                <span>
                  {pendingActions.includes(item.action)
                    ? 'Procesando…'
                    : userActionLabels[item.action]}
                </span>
                {item.disabledReason && (
                  <span className="ml-2 text-xs text-muted-foreground">
                    {item.disabledReason === 'last_admin'
                      ? 'Tiene que quedar al menos un administrador activo'
                      : 'Confirmá tu email para invitar a tu equipo.'}
                  </span>
                )}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </li>
  )
}
