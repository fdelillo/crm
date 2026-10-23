import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { LoaderCircle } from 'lucide-react'
import { Link, Navigate, Outlet, useLocation } from 'react-router'
import { usePermissionCheck, useSession } from '@/features/auth/session'
import { messageForError } from '@/api/errorMessages'
import { safeNextPath } from '@/lib/safeNextPath'
import { Button } from '@/components/ui/button'
import { ErrorState } from '@/components/ErrorState'
import { PageHeader } from '@/components/PageHeader'
import { ForbiddenState } from '@/components/ForbiddenState'
import {
  exitSession,
  resumeSessionExit,
  useSessionLifecycle,
} from '@/lib/sessionExit'
export function PublicOnly() {
  const { data } = useSession()
  const location = useLocation()
  return data ? (
    <Navigate
      to={safeNextPath(new URLSearchParams(location.search).get('next'))}
      replace
    />
  ) : (
    <Outlet />
  )
}

function LoadingSpinner() {
  return (
    <LoaderCircle
      aria-hidden="true"
      className="size-6 animate-spin motion-reduce:animate-none"
    />
  )
}

function SessionLoader() {
  return (
    <main role="status" aria-label="Cargando la aplicación" aria-busy="true">
      <span>CRM</span>
      <LoadingSpinner />
      <span>Cargando…</span>
    </main>
  )
}

// DD-F35, INV-F22 (ui.md §6.4.2): decides with the session-exit registry, never with the router state.
// During an exit the session is not read at all: the cache was just cleared, a new GET /me would
// only race the navigation to the login (and bounce it if the server still answered a session).
export function RequireSession() {
  const client = useQueryClient()
  const { exit } = useSessionLifecycle()
  const stalled = exit?.status === 'stalled'
  useEffect(() => {
    if (stalled) resumeSessionExit(client)
  }, [client, stalled])
  return exit ? <SessionLoader /> : <GuardedSession />
}

function GuardedSession() {
  const client = useQueryClient()
  const session = useSession()
  const location = useLocation()
  const { presented } = useSessionLifecycle()
  const leaving =
    !session.isPending && !session.isLoadingError && !session.data && presented
  useEffect(() => {
    if (leaving) exitSession(client, 'session_expired')
  }, [client, leaving])
  if (session.isPending) return <SessionLoader />
  if (session.isLoadingError) {
    const message = messageForError(session.error, 'generic')
    return (
      <main>
        <div role="alert">
          <h1>{message.title}</h1>
          <p>{message.description}</p>
          <Button
            onClick={() => void session.refetch()}
            disabled={session.isFetching}
          >
            Reintentar
          </Button>
        </div>
      </main>
    )
  }
  if (session.data) return <Outlet />
  if (presented) return <SessionLoader />
  return (
    <Navigate
      to={`/login?next=${encodeURIComponent(location.pathname + location.search)}`}
      replace
    />
  )
}
export function RequirePermission() {
  const location = useLocation()
  const check = usePermissionCheck('settings.manage', location.pathname)
  if (check.status === 'granted') return <Outlet />
  if (check.status === 'denied') return <ForbiddenState />
  if (check.status === 'failed')
    return (
      <>
        <PageHeader title="No pudimos comprobar tu acceso" />
        <ErrorState
          message={messageForError(check.error, 'generic')}
          onRetry={check.retry}
        />
        <Link to="/">Volver al inicio</Link>
      </>
    )
  return (
    <div role="status" aria-label="Comprobando tu acceso" aria-busy="true">
      <LoadingSpinner />
      <span>Comprobando tu acceso…</span>
    </div>
  )
}
