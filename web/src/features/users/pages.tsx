import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router'
import { toast } from 'sonner'
import { isApiError } from '@/api/errors'
import { messageForError } from '@/api/errorMessages'
import { queryKeys } from '@/api/queryKeys'
import type { User } from '@/api/types'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { PageHeader } from '@/components/PageHeader'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { VerifyToInviteNotice } from '@/components/VerifyToInviteNotice'
import { useCurrentSession, useInviteBlockedReason, useResendEmailVerification } from '@/features/auth/session'
import { useChangeUserRole, useDeactivateUser, useInviteUser, useReactivateUser, useUsers } from './api'
import { availableUserActions, countActiveAdmins, type UserAction } from './rules'
import { UserListItem } from './components/UserListItem'

const names: Record<UserAction, string> = {
  makeAdmin: 'Hacer Administrador',
  makeOperator: 'Cambiar a Operador',
  deactivate: 'Desactivar',
  reactivate: 'Reactivar',
  resendInvitation: 'Reenviar invitación',
}

function userName(user: User) {
  return user.name ?? user.email
}

function confirmation(user: User, action: UserAction, self: boolean) {
  const name = userName(user)
  if (action === 'makeAdmin') return { title: `¿Hacer Administrador a ${name}?`, description: user.status === 'invited' ? 'Cuando acepte la invitación, va a poder ver toda la información de la empresa y gestionar usuarios.' : 'Va a poder ver toda la información de la empresa y gestionar usuarios.' }
  if (action === 'makeOperator') return { title: `¿Cambiar a ${name} a Operador?`, description: self ? 'Vas a perder el acceso a Ajustes.' : 'Deja de ver finanzas, reportes y ajustes.' }
  if (action === 'deactivate') return { title: `¿Desactivar a ${name}?`, description: self ? 'Se va a cerrar tu sesión.' : user.status === 'invited' ? 'Su invitación deja de funcionar.' : 'No va a poder ingresar y se cierran sus sesiones abiertas. Podés reactivarlo después.' }
  return { title: `¿Reactivar a ${name}?`, description: 'Va a poder ingresar de nuevo con su contraseña. Si nunca aceptó la invitación, le enviamos una nueva.' }
}

export function UsersPage() {
  const session = useCurrentSession()
  const users = useUsers()
  const queryClient = useQueryClient()
  const blockedReason = useInviteBlockedReason()
  const resend = useResendEmailVerification()
  const invite = useInviteUser()
  const changeRole = useChangeUserRole()
  const deactivate = useDeactivateUser()
  const reactivate = useReactivateUser()
  const [target, setTarget] = useState<{ user: User; action: UserAction; trigger: HTMLButtonElement } | null>(null)
  const [pendingAction, setPendingAction] = useState<UserAction | null>(null)
  const submitLock = useRef(false)
  const restoreTrigger = useRef<HTMLButtonElement | null>(null)

  useEffect(() => {
    if (!target && restoreTrigger.current) {
      restoreTrigger.current.focus()
      restoreTrigger.current = null
    }
  }, [target])

  const resendStatus = resend.isPending ? 'pending' : resend.isError ? 'error' : resend.data ?? 'idle'
  const resendError = resend.error ? messageForError(resend.error, 'resendVerification') : undefined
  const sendVerification = () => {
    if (!resend.isPending) resend.mutate()
  }
  const list = users.data ?? []
  const activeAdminCount = countActiveAdmins(list)

  async function runAction(user: User, action: UserAction) {
    if (submitLock.current) return
    submitLock.current = true
    setPendingAction(action)
    const displayName = userName(user)
    try {
      if (action === 'makeAdmin' || action === 'makeOperator') {
        const changed = await changeRole.mutateAsync({ id: user.id, role: action === 'makeAdmin' ? 'admin' : 'operator' })
        toast(`${displayName} ahora es ${changed.role === 'admin' ? 'Administrador' : 'Operador'}`)
        if (user.id === session.user.id) await queryClient.invalidateQueries({ queryKey: queryKeys.session, exact: true })
      } else if (action === 'deactivate') {
        await deactivate.mutateAsync(user.id)
        toast(`Desactivaste a ${displayName}`)
      } else if (action === 'reactivate') {
        const updated = await reactivate.mutateAsync(user.id)
        toast(updated.status === 'invited' ? `Le enviamos una invitación nueva a ${user.email}` : `Reactivaste a ${displayName}`)
      } else {
        await invite.mutateAsync({ email: user.email, role: user.role })
        toast(`Reenviamos la invitación a ${user.email}`)
      }
      if (target) restoreTrigger.current = target.trigger
      setTarget(null)
    } catch (error) {
      if (isApiError(error) && error.code === 'email_not_verified') {
        toast(messageForError(error, 'invite').title)
        await queryClient.invalidateQueries({ queryKey: queryKeys.session, exact: true })
      } else {
        const message = messageForError(error, 'userAction')
        toast(message.description ? `${message.title} ${message.description}` : message.title)
        if (isApiError(error) && ['last_admin', 'invalid_state', 'not_found'].includes(error.code ?? '')) {
          await users.refetch()
        }
      }
      if (target) restoreTrigger.current = target.trigger
      setTarget(null)
    } finally {
      submitLock.current = false
      setPendingAction(null)
    }
  }

  function onAction(user: User, action: UserAction, trigger: HTMLButtonElement) {
    if (action === 'resendInvitation') void runAction(user, action)
    else setTarget({ user, action, trigger })
  }

  const selectedConfirmation = target ? confirmation(target.user, target.action, target.user.id === session.user.id) : null
  return (
    <>
      <PageHeader
        title="Usuarios"
        back={{ to: '/settings', label: '← Ajustes' }}
        actions={blockedReason ? (
          <Button type="button" disabled aria-describedby="verify-to-invite-notice">+ Invitar</Button>
        ) : (
          <Button asChild><Link to="/settings/users/invite">+ Invitar</Link></Button>
        )}
      />
      {blockedReason && <VerifyToInviteNotice id="verify-to-invite-notice" email={session.user.email} resendStatus={resendStatus} errorMessage={resendError} onResend={sendVerification} />}
      {users.isPending ? (
        <section role="region" aria-label="Cargando usuarios" aria-busy="true" className="space-y-3">
          {[0, 1, 2].map((item) => <Skeleton key={item} aria-hidden="true" className="h-24 w-full" />)}
        </section>
      ) : users.isError ? (
        <ErrorState message={{ ...messageForError(users.error, 'users'), title: 'No pudimos cargar los usuarios.' }} onRetry={() => void users.refetch()} retrying={users.isFetching} />
      ) : (
        <>
          <ul className="space-y-3">
            {list.map((user) => (
              <UserListItem key={user.id} user={user} isCurrentUser={user.id === session.user.id} actions={availableUserActions(user, { currentUserId: session.user.id, activeAdminCount, actorEmailVerified: session.user.email_verified })} timeZone={session.tenant.timezone} pendingAction={pendingAction} onAction={(action, trigger) => onAction(user, action, trigger)} />
            ))}
          </ul>
          {list.length === 1 && list[0]?.id === session.user.id && (
            blockedReason ? (
              <section className="mt-4 space-y-3 rounded-lg border bg-card p-6"><h2 className="text-lg font-semibold">Todavía sos el único usuario.</h2><p className="text-muted-foreground">Invitá a alguien de tu equipo para que cargue clientes y proyectos.</p><Button disabled aria-describedby="verify-to-invite-notice">Invitar</Button></section>
            ) : <div className="mt-4"><EmptyState title="Todavía sos el único usuario." description="Invitá a alguien de tu equipo para que cargue clientes y proyectos." action={{ label: 'Invitar', to: '/settings/users/invite' }} /></div>
          )}
        </>
      )}
      {target && selectedConfirmation && (
        <ConfirmDialog open onOpenChange={(open) => { if (!open && !pendingAction) { restoreTrigger.current = target.trigger; setTarget(null) } }} title={selectedConfirmation.title} description={selectedConfirmation.description} confirmLabel={names[target.action]} pendingLabel={`${names[target.action]}…`} destructive={target.action === 'deactivate'} pending={pendingAction === target.action} onConfirm={() => void runAction(target.user, target.action)} />
      )}
    </>
  )
}

export function InviteUserPage() {
  const navigate = useNavigate()
  return <><PageHeader title="Invitar a alguien" back={{ to: '/settings/users', label: '← Usuarios' }} description="Le mandamos un email para que cree su contraseña. La invitación vence en 7 días." /><Link to="/settings/users">Cancelar</Link></>
}
