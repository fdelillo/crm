import { useState } from 'react'
import { LoaderCircle } from 'lucide-react'
import {
  Link,
  Navigate,
  Outlet,
  useLocation,
  useNavigation,
} from 'react-router'
import { usePermissionCheck, useSession } from '@/features/auth/session'
import { messageForError } from '@/api/errorMessages'
import { safeNextPath } from '@/lib/safeNextPath'
import { Button } from '@/components/ui/button'
import { ErrorState } from '@/components/ErrorState'
import { PageHeader } from '@/components/PageHeader'
import { ForbiddenState } from '@/components/ForbiddenState'
import { isSessionExpiredRedirect } from '@/lib/sessionExpiredRedirect'
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

export function RequireSession() {
  const session = useSession()
  const location = useLocation()
  const navigation = useNavigation()
  const [wasAuthenticated, setWasAuthenticated] = useState(
    Boolean(session.data),
  )
  if (session.data && !wasAuthenticated) setWasAuthenticated(true)
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
  // Keep intentional login destinations while their lazy chunk loads. The
  // session query is cleared during these navigations, so a plain Navigate
  // here would race them and drop their reason.
  if (
    !session.data &&
    navigation.location &&
    navigation.location.pathname === '/login' &&
    (isSessionExpiredRedirect(navigation.location) ||
      new URLSearchParams(navigation.location.search).get('reason') ===
        'logged_out')
  )
    return <SessionLoader />
  // Cache clear can publish a fresh null before the router's pending snapshot.
  // Once authenticated, wait for the existing session/logout handler to navigate.
  if (!session.data && wasAuthenticated) return <SessionLoader />
  if (!session.data)
    return (
      <Navigate
        to={`/login?next=${encodeURIComponent(location.pathname + location.search)}`}
        replace
      />
    )
  return <Outlet />
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
