import { LoaderCircle } from 'lucide-react'
import { Navigate, Outlet, useLocation, useNavigation } from 'react-router'
import { useCan, useSession } from '@/features/auth/session'
import { messageForError } from '@/api/errorMessages'
import { safeNextPath } from '@/lib/safeNextPath'
import { Button } from '@/components/ui/button'
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
export function RequireSession() {
  const session = useSession()
  const location = useLocation()
  const navigation = useNavigation()
  if (session.isPending)
    return (
      <main role="status" aria-label="Cargando la aplicación" aria-busy="true">
        <span>CRM</span>
        <LoaderCircle
          aria-hidden="true"
          className="size-6 animate-spin motion-reduce:animate-none"
        />
        <span>Cargando…</span>
      </main>
    )
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
    return (
      <main role="status" aria-label="Cargando la aplicación" aria-busy="true">
        <span>CRM</span>
        <LoaderCircle
          aria-hidden="true"
          className="size-6 animate-spin motion-reduce:animate-none"
        />
        <span>Cargando…</span>
      </main>
    )
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
  return useCan('settings.manage') ? <Outlet /> : <ForbiddenState />
}
