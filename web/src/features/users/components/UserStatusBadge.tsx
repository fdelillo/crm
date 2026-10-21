import { Badge } from '@/components/ui/badge'
import { formatDateTime } from '@/lib/format'
import type { UserStatus } from '@/api/types'

const labels: Record<UserStatus, string> = {
  active: 'Activo',
  invited: 'Invitado',
  disabled: 'Desactivado',
}

export interface UserStatusBadgeProps {
  status: UserStatus
  invitationExpiresAt: string | null
  timeZone: string
  now?: Date
}

export function UserStatusBadge({
  status,
  invitationExpiresAt,
  timeZone,
  now = new Date(),
}: UserStatusBadgeProps) {
  const expired =
    status === 'invited' &&
    invitationExpiresAt !== null &&
    Date.parse(invitationExpiresAt) < now.getTime()
  return (
    <span className="inline-flex flex-wrap items-center gap-2">
      <span>·</span>
      <Badge
        variant={
          status === 'active'
            ? 'secondary'
            : status === 'disabled'
              ? 'outline'
              : 'default'
        }
      >
        {labels[status]}
      </Badge>
      {status === 'invited' && invitationExpiresAt && (
        <span className="w-full text-sm text-muted-foreground">
          {expired ? 'La invitación venció el ' : 'La invitación vence el '}
          {formatDateTime(invitationExpiresAt, timeZone, now)}
          {expired ? '. Reenviala para que pueda entrar.' : ''}
        </span>
      )}
    </span>
  )
}
