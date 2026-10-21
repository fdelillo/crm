import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { zodResolver } from '@hookform/resolvers/zod'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router'
import { toast } from 'sonner'
import { isApiError } from '@/api/errors'
import {
  applyServerFieldErrors,
  fieldErrorText,
  messageForError,
} from '@/api/errorMessages'
import { queryKeys } from '@/api/queryKeys'
import { formatDateTime } from '@/lib/format'
import type { User } from '@/api/types'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { FormAlert } from '@/components/FormAlert'
import { PageHeader } from '@/components/PageHeader'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { SubmitButton } from '@/components/SubmitButton'
import { VerifyToInviteNotice } from '@/components/VerifyToInviteNotice'
import {
  useInviteBlockedReason,
  useCurrentSession,
  useVerifyToInviteNotice,
} from '@/features/auth/session'
import {
  useChangeUserRole,
  useDeactivateUser,
  useInviteUser,
  useReactivateUser,
  useUsers,
} from './api'
import {
  availableUserActions,
  countActiveAdmins,
  userActionLabels,
  type UserAction,
  type UserActionAvailability,
} from './rules'
import { UserListItem } from './components/UserListItem'
import { invitationSchema, type InvitationFormValues } from './schemas'

function userName(user: User) {
  return user.name ?? user.email
}

function actionKey(userId: string, action: UserAction) {
  return `${userId}:${action}`
}

function confirmation(
  user: User,
  action: UserAction,
  selfWarning?: UserActionAvailability['selfWarning'],
) {
  const name = userName(user)
  if (action === 'makeAdmin')
    return {
      title: `¿Hacer Administrador a ${name}?`,
      description:
        user.status === 'invited'
          ? 'Cuando acepte la invitación, va a poder ver toda la información de la empresa y gestionar usuarios.'
          : 'Va a poder ver toda la información de la empresa y gestionar usuarios.',
    }
  if (action === 'makeOperator')
    return {
      title: `¿Cambiar a ${name} a Operador?`,
      description:
        selfWarning === 'lose_settings_access'
          ? 'Vas a perder el acceso a Ajustes.'
          : 'Deja de ver finanzas, reportes y ajustes.',
    }
  if (action === 'deactivate')
    return {
      title: `¿Desactivar a ${name}?`,
      description:
        selfWarning === 'end_own_session'
          ? 'Se va a cerrar tu sesión.'
          : user.status === 'invited'
            ? 'Su invitación deja de funcionar.'
            : 'No va a poder ingresar y se cierran sus sesiones abiertas. Podés reactivarlo después.',
    }
  return {
    title: `¿Reactivar a ${name}?`,
    description:
      'Va a poder ingresar de nuevo con su contraseña. Si nunca aceptó la invitación, le enviamos una nueva.',
  }
}

export function UsersPage() {
  const currentSession = useCurrentSession()
  const users = useUsers()
  const queryClient = useQueryClient()
  const blockedReason = useInviteBlockedReason()
  const verificationNotice = useVerifyToInviteNotice()
  const resend = verificationNotice.resend
  const invite = useInviteUser()
  const changeRole = useChangeUserRole()
  const deactivate = useDeactivateUser()
  const reactivate = useReactivateUser()
  const [target, setTarget] = useState<{
    user: User
    action: UserAction
    trigger: HTMLButtonElement
  } | null>(null)
  const targetRef = useRef(target)
  useLayoutEffect(() => {
    targetRef.current = target
  }, [target])
  const [pendingActions, setPendingActions] = useState<Set<string>>(
    () => new Set(),
  )
  const submitLocks = useRef(new Set<string>())
  const restoreTrigger = useRef<HTMLButtonElement | null>(null)

  useEffect(() => {
    if (!target && restoreTrigger.current) {
      restoreTrigger.current.focus()
      restoreTrigger.current = null
    }
  }, [target])

  const sendVerification = () => {
    if (!resend.isPending) void verificationNotice.sendAgain()
  }
  const list = users.data ?? []
  const activeAdminCount = countActiveAdmins(list)

  async function runAction(user: User, action: UserAction) {
    const key = actionKey(user.id, action)
    if (submitLocks.current.has(key)) return
    submitLocks.current.add(key)
    setPendingActions((current) => new Set(current).add(key))
    const targetAtStart = target
    const closeTarget = () => {
      if (targetAtStart && targetRef.current === targetAtStart) {
        restoreTrigger.current = targetAtStart.trigger
        setTarget(null)
      }
    }
    const displayName = userName(user)
    try {
      if (action === 'makeAdmin' || action === 'makeOperator') {
        const changed = await changeRole.mutateAsync({
          userId: user.id,
          role: action === 'makeAdmin' ? 'admin' : 'operator',
        })
        toast(
          `${displayName} ahora es ${changed.role === 'admin' ? 'Administrador' : 'Operador'}`,
        )
        if (user.id === currentSession.user.id)
          await queryClient.invalidateQueries({
            queryKey: queryKeys.session,
            exact: true,
          })
      } else if (action === 'deactivate') {
        await deactivate.mutateAsync({ userId: user.id })
        toast(`Desactivaste a ${displayName}`)
      } else if (action === 'reactivate') {
        const updated = await reactivate.mutateAsync({ userId: user.id })
        toast(
          updated.status === 'invited'
            ? `Le enviamos una invitación nueva a ${user.email}`
            : `Reactivaste a ${displayName}`,
        )
      } else {
        await invite.mutateAsync({ email: user.email, role: user.role })
        toast(`Reenviamos la invitación a ${user.email}`)
      }
      closeTarget()
    } catch (error) {
      if (isApiError(error) && error.code === 'email_not_verified') {
        toast(messageForError(error, 'invite').title)
        await queryClient.invalidateQueries({
          queryKey: queryKeys.session,
          exact: true,
        })
      } else if (
        !isApiError(error) ||
        (error.code !== 'unauthenticated' && error.code !== 'forbidden')
      ) {
        const message = messageForError(error, 'userAction')
        toast(
          message.description
            ? `${message.title} ${message.description}`
            : message.title,
        )
        if (
          isApiError(error) &&
          ['last_admin', 'invalid_state', 'not_found'].includes(
            error.code ?? '',
          )
        ) {
          await users.refetch()
        }
      } else {
        // 401 and forbidden 403 are already handled by the global session layer.
      }
      closeTarget()
    } finally {
      submitLocks.current.delete(key)
      setPendingActions((current) => {
        const next = new Set(current)
        next.delete(key)
        return next
      })
    }
  }

  function onAction(
    user: User,
    action: UserAction,
    trigger: HTMLButtonElement,
  ) {
    if (action === 'resendInvitation') void runAction(user, action)
    else setTarget({ user, action, trigger })
  }

  const selectedConfirmation = target
    ? confirmation(
        target.user,
        target.action,
        availableUserActions(target.user, {
          currentUserId: currentSession.user.id,
          activeAdminCount,
          actorEmailVerified: currentSession.user.email_verified,
        }).find((action) => action.action === target.action)?.selfWarning,
      )
    : null
  return (
    <>
      <PageHeader
        title="Usuarios"
        back={{ to: '/settings', label: '← Ajustes' }}
        actions={
          blockedReason ? (
            <Button
              type="button"
              disabled
              aria-describedby="verify-to-invite-notice"
            >
              + Invitar
            </Button>
          ) : (
            <Button asChild>
              <Link to="/settings/users/invite">+ Invitar</Link>
            </Button>
          )
        }
      />
      {blockedReason && (
        <VerifyToInviteNotice
          id="verify-to-invite-notice"
          email={currentSession.user.email}
          resendStatus={verificationNotice.status}
          errorMessage={verificationNotice.errorMessage}
          onResend={sendVerification}
        />
      )}
      {users.isPending ? (
        <section
          aria-label="Cargando usuarios"
          aria-busy="true"
          className="space-y-3"
        >
          {[0, 1, 2].map((item) => (
            <Skeleton key={item} aria-hidden="true" className="h-24 w-full" />
          ))}
        </section>
      ) : users.isError ? (
        <ErrorState
          message={{
            ...messageForError(users.error, 'users'),
            title: 'No pudimos cargar los usuarios.',
          }}
          onRetry={() => void users.refetch()}
          retrying={users.isFetching}
        />
      ) : (
        <>
          <ul className="space-y-3">
            {list.map((user) => (
              <UserListItem
                key={user.id}
                user={user}
                isCurrentUser={user.id === currentSession.user.id}
                actions={availableUserActions(user, {
                  currentUserId: currentSession.user.id,
                  activeAdminCount,
                  actorEmailVerified: currentSession.user.email_verified,
                })}
                timeZone={currentSession.tenant.timezone}
                pendingActions={Array.from(pendingActions)
                  .filter((key) => key.startsWith(`${user.id}:`))
                  .map((key) => key.slice(user.id.length + 1) as UserAction)}
                onAction={(action, trigger) => onAction(user, action, trigger)}
              />
            ))}
          </ul>
          {list.length === 1 &&
            list[0]?.id === currentSession.user.id &&
            (blockedReason ? (
              <div className="mt-4">
                <EmptyState
                  title="Todavía sos el único usuario."
                  description="Invitá a alguien de tu equipo para que cargue clientes y proyectos."
                  action={{
                    label: 'Invitar',
                    to: '/settings/users/invite',
                    disabled: true,
                    ariaDescribedBy: 'verify-to-invite-notice',
                  }}
                />
              </div>
            ) : (
              <div className="mt-4">
                <EmptyState
                  title="Todavía sos el único usuario."
                  description="Invitá a alguien de tu equipo para que cargue clientes y proyectos."
                  action={{ label: 'Invitar', to: '/settings/users/invite' }}
                />
              </div>
            ))}
        </>
      )}
      {target && selectedConfirmation && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (
              !open &&
              !pendingActions.has(actionKey(target.user.id, target.action))
            ) {
              restoreTrigger.current = target.trigger
              setTarget(null)
            }
          }}
          title={selectedConfirmation.title}
          description={selectedConfirmation.description}
          confirmLabel={userActionLabels[target.action]}
          pendingLabel={`${userActionLabels[target.action]}…`}
          destructive={target.action === 'deactivate'}
          pending={pendingActions.has(actionKey(target.user.id, target.action))}
          onConfirm={() => void runAction(target.user, target.action)}
        />
      )}
    </>
  )
}

export function InviteUserPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const currentSession = useCurrentSession()
  const blockedReason = useInviteBlockedReason()
  const verificationNotice = useVerifyToInviteNotice()
  const resend = verificationNotice.resend
  const invitation = useInviteUser()
  const [submitError, setSubmitError] = useState<unknown>(null)
  const submitLock = useRef(false)
  const form = useForm<InvitationFormValues>({
    resolver: zodResolver(invitationSchema),
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    defaultValues: { email: '', role: 'operator' },
  })
  const {
    register,
    control,
    handleSubmit,
    setError,
    setFocus,
    clearErrors,
    formState: { errors },
  } = form
  const cachedUsers = queryClient.getQueryData<User[]>(queryKeys.users)
  const emailErrorId = errors.email ? 'invite-email-error' : undefined
  const roleErrorId = errors.role ? 'invite-role-error' : undefined
  const blocked = Boolean(blockedReason)

  async function onSubmit(values: InvitationFormValues) {
    if (submitLock.current || blocked) return
    submitLock.current = true
    setSubmitError(null)
    clearErrors()
    try {
      const email = values.email
      const previousInvite = cachedUsers?.find(
        (user) =>
          user.email.toLowerCase() === email && user.status === 'invited',
      )
      const { user: created, reissued } = await invitation.mutateAsync(values)
      if (reissued) {
        const roleText =
          previousInvite && previousInvite.role !== created.role
            ? ` como ${created.role === 'admin' ? 'Administrador' : 'Operador'}`
            : ''
        toast(
          `${email} ya estaba invitado: le reenviamos la invitación${roleText}.`,
        )
      } else {
        const expiry = created.invitation_expires_at
          ? ` Vence el ${formatDateTime(
              created.invitation_expires_at,
              currentSession.tenant.timezone,
            )}.`
          : ''
        toast(`Invitación enviada a ${email}.${expiry}`)
      }
      await navigate('/settings/users', { replace: true })
    } catch (error) {
      if (isApiError(error) && error.code === 'email_taken') {
        const disabledUser = cachedUsers?.find(
          (user) =>
            user.email.toLowerCase() ===
              form.getValues('email').trim().toLowerCase() &&
            user.status === 'disabled',
        )
        const text = disabledUser
          ? `Es de ${disabledUser.name ?? disabledUser.email}, que está desactivado. Podés reactivarlo desde la lista.`
          : 'Ese email ya tiene un usuario en el sistema.'
        setError('email', { type: 'server', message: text })
        setFocus('email')
        return
      }
      if (isApiError(error) && error.code === 'email_not_verified') {
        setSubmitError(error)
        await queryClient.invalidateQueries({
          queryKey: queryKeys.session,
          exact: true,
        })
        if (queryClient.getQueryState(queryKeys.session)?.status === 'success')
          setSubmitError(null)
        return
      }
      if (isApiError(error) && error.code === 'validation_failed') {
        const fields = ['email', 'role'] as const
        const unknownErrors = applyServerFieldErrors(
          setError,
          error.fieldErrors,
          fields,
        )
        const firstField = fields.find((field) =>
          error.fieldErrors.some((item) => item.field === field),
        )
        if (firstField) setFocus(firstField)
        if (error.fieldErrors.length && unknownErrors.length === 0) return
      }
      setSubmitError(error)
    } finally {
      submitLock.current = false
    }
  }

  const message = submitError ? messageForError(submitError, 'invite') : null
  const verificationError =
    isApiError(submitError) && submitError.code === 'email_not_verified'
  const retrySubmit = () => void handleSubmit(onSubmit)()
  return (
    <>
      <PageHeader
        title="Invitar a alguien"
        back={{ to: '/settings/users', label: '← Usuarios' }}
        description="Le mandamos un email para que cree su contraseña. La invitación vence en 7 días."
      />
      {blocked ? (
        <div className="space-y-4">
          <VerifyToInviteNotice
            id="verify-to-invite-notice"
            email={currentSession.user.email}
            resendStatus={verificationNotice.status}
            errorMessage={verificationNotice.errorMessage}
            onResend={() => {
              if (!resend.isPending) void verificationNotice.sendAgain()
            }}
          />
          <Link
            className="inline-flex min-h-11 items-center text-primary underline"
            to="/settings/users"
          >
            Volver a Usuarios
          </Link>
        </div>
      ) : (
        <form
          className="space-y-5"
          noValidate
          onSubmit={handleSubmit(onSubmit)}
        >
          <Field data-invalid={Boolean(errors.email)}>
            <FieldLabel htmlFor="invite-email">Email</FieldLabel>
            <Input
              id="invite-email"
              type="email"
              autoComplete="email"
              className="h-11"
              readOnly={invitation.isPending}
              aria-invalid={Boolean(errors.email)}
              aria-describedby={emailErrorId}
              {...register('email', {
                onChange: () => {
                  clearErrors('email')
                  setSubmitError(null)
                },
              })}
            />
            {errors.email && (
              <FieldError id={emailErrorId}>
                {fieldErrorText('email', errors.email)}
              </FieldError>
            )}
          </Field>
          <Field data-invalid={Boolean(errors.role)}>
            <FieldLabel id="invite-role-label">Rol</FieldLabel>
            <Controller
              control={control}
              name="role"
              render={({ field }) => (
                <RadioGroup
                  aria-labelledby="invite-role-label"
                  value={field.value}
                  onValueChange={(value) => {
                    field.onChange(value)
                    clearErrors('role')
                  }}
                  aria-invalid={Boolean(errors.role)}
                  aria-describedby={roleErrorId}
                >
                  <label
                    htmlFor="invite-role-operator"
                    className="flex min-h-11 cursor-pointer items-start gap-3 rounded-md border p-3"
                  >
                    <RadioGroupItem
                      id="invite-role-operator"
                      value="operator"
                      disabled={invitation.isPending}
                    />
                    <span>
                      <span className="font-medium">Operador</span>
                      <FieldDescription>
                        Carga clientes, proyectos, presupuestos y cobros. No ve
                        finanzas ni ajustes.
                      </FieldDescription>
                    </span>
                  </label>
                  <label
                    htmlFor="invite-role-admin"
                    className="flex min-h-11 cursor-pointer items-start gap-3 rounded-md border p-3"
                  >
                    <RadioGroupItem
                      id="invite-role-admin"
                      value="admin"
                      disabled={invitation.isPending}
                    />
                    <span>
                      <span className="font-medium">Administrador</span>
                      <FieldDescription>Ve y gestiona todo.</FieldDescription>
                    </span>
                  </label>
                </RadioGroup>
              )}
            />
            {errors.role && (
              <FieldError id={roleErrorId}>
                {fieldErrorText('role', errors.role)}
              </FieldError>
            )}
          </Field>
          {verificationError && message ? (
            <FormAlert
              variant="warning"
              title={message.title}
              description={message.description}
              action={{
                label: 'Reenviar email de confirmación',
                onClick: () => {
                  if (!resend.isPending) void verificationNotice.sendAgain()
                },
                disabled: resend.isPending,
              }}
            />
          ) : message ? (
            <FormAlert
              variant="error"
              title={message.title}
              description={message.description}
              requestId={message.requestId}
              action={
                message.retryable
                  ? { label: 'Reintentar', onClick: retrySubmit }
                  : undefined
              }
            />
          ) : null}
          <div className="flex flex-wrap gap-3">
            <SubmitButton
              pending={invitation.isPending}
              pendingLabel="Enviando…"
              className="h-11"
            >
              Enviar invitación
            </SubmitButton>
            <Button type="button" variant="outline" asChild>
              <Link to="/settings/users">Cancelar</Link>
            </Button>
          </div>
        </form>
      )}
    </>
  )
}
