import type { User } from '@/api/types'

export type UserAction =
  | 'makeAdmin'
  | 'makeOperator'
  | 'deactivate'
  | 'reactivate'
  | 'resendInvitation'

export const userActionLabels: Record<UserAction, string> = {
  makeAdmin: 'Hacer Administrador',
  makeOperator: 'Cambiar a Operador',
  deactivate: 'Desactivar',
  reactivate: 'Reactivar',
  resendInvitation: 'Reenviar invitación',
}

export interface UserActionAvailability {
  action: UserAction
  enabled: boolean
  disabledReason?: 'last_admin' | 'email_not_verified'
  requiresConfirmation: boolean
  selfWarning?: 'lose_settings_access' | 'end_own_session'
}

export function countActiveAdmins(users: User[]): number {
  return users.filter(
    (user) => user.status === 'active' && user.role === 'admin',
  ).length
}

export function availableUserActions(
  user: User,
  ctx: {
    currentUserId: string
    activeAdminCount: number
    actorEmailVerified: boolean
  },
): UserActionAvailability[] {
  const self = user.id === ctx.currentUserId
  const lastAdmin =
    user.status === 'active' &&
    user.role === 'admin' &&
    ctx.activeAdminCount <= 1
  const confirmed = ctx.actorEmailVerified

  if (user.status === 'disabled') {
    return [
      {
        action: 'reactivate',
        enabled: confirmed || user.name !== null,
        ...(!confirmed && user.name === null
          ? { disabledReason: 'email_not_verified' as const }
          : {}),
        requiresConfirmation: true,
      },
    ]
  }

  if (user.status === 'invited') {
    const actions: UserActionAvailability[] = []
    if (user.role === 'admin') {
      actions.push({
        action: 'resendInvitation',
        enabled: confirmed,
        ...(!confirmed
          ? { disabledReason: 'email_not_verified' as const }
          : {}),
        requiresConfirmation: false,
      })
      actions.push({
        action: 'makeOperator',
        enabled: true,
        requiresConfirmation: true,
      })
    } else {
      actions.push({
        action: 'resendInvitation',
        enabled: confirmed,
        ...(!confirmed
          ? { disabledReason: 'email_not_verified' as const }
          : {}),
        requiresConfirmation: false,
      })
    }
    if (user.role === 'operator') {
      actions.push({
        action: 'makeAdmin',
        enabled: true,
        requiresConfirmation: true,
      })
    }
    actions.push({
      action: 'deactivate',
      enabled: true,
      requiresConfirmation: true,
    })
    return actions
  }

  const roleAction = user.role === 'admin' ? 'makeOperator' : 'makeAdmin'
  return [
    {
      action: roleAction,
      enabled: !lastAdmin,
      ...(lastAdmin ? { disabledReason: 'last_admin' as const } : {}),
      requiresConfirmation: true,
      ...(self && !lastAdmin && user.role === 'admin'
        ? { selfWarning: 'lose_settings_access' as const }
        : {}),
    },
    {
      action: 'deactivate',
      enabled: !lastAdmin,
      ...(lastAdmin ? { disabledReason: 'last_admin' as const } : {}),
      requiresConfirmation: true,
      ...(self && !lastAdmin
        ? { selfWarning: 'end_own_session' as const }
        : {}),
    },
  ]
}
