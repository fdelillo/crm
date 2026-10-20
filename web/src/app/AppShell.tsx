import type { SessionInfo } from '@/api/types'
import { useEffect, useRef, useState } from 'react'
import { useTenant, logoUrl } from '@/api/tenant'
import { Home, Settings, ChevronDown } from 'lucide-react'
import { NavLink, Outlet } from 'react-router'
import { useResendEmailVerification, useSession } from '@/features/auth/session'
import { AppBrand } from '@/components/AppBrand'
import { Button } from '@/components/ui/button'
import { useLogout } from '@/features/auth/api'
import { messageForError } from '@/api/errorMessages'
import {
  EmailVerificationBanner,
  useEmailVerificationDismissed,
  type EmailVerificationNoticeStatus,
} from '@/components/EmailVerificationBanner'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuItem,
} from '@/components/ui/dropdown-menu'
export function AppShell() {
  const { data: session } = useSession()
  return session ? <AuthenticatedShell session={session} /> : null
}
function AuthenticatedShell({ session }: { session: SessionInfo }) {
  const logout = useLogout()
  const [logoutStarted, setLogoutStarted] = useState(false)
  const { data: tenant } = useTenant({
    enabled: !logout.isPending && !logoutStarted,
  })
  const role = session.user.role === 'admin' ? 'Administrador' : 'Operador'
  const previouslyVerified = useRef(session.user.email_verified)
  useEffect(() => {
    if (!previouslyVerified.current && session.user.email_verified) {
      document.querySelector<HTMLElement>('#main h1')?.focus()
    }
    previouslyVerified.current = session.user.email_verified
  }, [session.user.email_verified])
  return (
    <div className="min-h-dvh lg:pl-60">
      <a
        href="#main"
        className="fixed left-4 top-4 z-[200] -translate-y-24 rounded-lg bg-primary px-4 py-3 text-primary-foreground focus:translate-y-0"
        onClick={() => document.getElementById('main')?.focus()}
      >
        Saltar al contenido
      </a>
      <div className="contents lg:fixed lg:inset-y-0 lg:left-0 lg:flex lg:w-60 lg:flex-col lg:overflow-y-auto lg:border-r lg:bg-card">
        <header className="border-b bg-card px-4 py-3 lg:border-b-0 lg:p-6">
          <AppBrand
            name={tenant?.name ?? session.tenant.name}
            logoSrc={tenant ? logoUrl(tenant) : null}
          />
        </header>
        <nav
          aria-label="Navegación principal"
          className="fixed inset-x-0 bottom-0 z-40 flex justify-around gap-2 border-t bg-card px-4 pb-[env(safe-area-inset-bottom)] lg:static lg:flex-col lg:justify-start lg:border-0 lg:px-4 lg:pb-4 lg:pt-0"
        >
          {[
            { to: '/', label: 'Inicio', Icon: Home },
            { to: '/settings', label: 'Ajustes', Icon: Settings },
          ].map(({ to, label, Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                `flex min-h-14 flex-1 items-center justify-center gap-2 rounded-lg px-4 py-2 lg:justify-start ${isActive ? 'bg-accent text-accent-foreground' : 'text-muted-foreground'}`
              }
            >
              <Icon size={20} aria-hidden="true" />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="hidden lg:mt-auto lg:block lg:p-4">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                className="h-auto w-full justify-between whitespace-normal text-left"
              >
                {session.user.name} · {role}
                <ChevronDown aria-hidden="true" size={16} />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuLabel>
                {session.user.name} · {role}
              </DropdownMenuLabel>
              <DropdownMenuItem
                disabled={logout.isPending || logoutStarted}
                onSelect={(event) => {
                  event.preventDefault()
                  setLogoutStarted(true)
                  logout.mutate(undefined, {
                    onError: () => setLogoutStarted(false),
                  })
                }}
              >
                {logout.isPending || logoutStarted
                  ? 'Cerrando sesión…'
                  : 'Cerrar sesión'}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      {!session.user.email_verified && (
        <SessionVerificationBanner
          key={session.user.email}
          email={session.user.email}
        />
      )}
      <main
        id="main"
        tabIndex={-1}
        className="mx-auto max-w-3xl px-4 pt-6 pb-[calc(5.5rem+env(safe-area-inset-bottom))] md:px-6 lg:py-8"
      >
        <Outlet />
      </main>
    </div>
  )
}

function SessionVerificationBanner({ email }: { email: string }) {
  const resend = useResendEmailVerification()
  const [dismissed, dismiss] = useEmailVerificationDismissed(email)
  const [outcome, setOutcome] = useState<'sent' | 'unknown' | null>(null)
  const [error, setError] = useState<unknown>(null)
  const status: EmailVerificationNoticeStatus = resend.isPending
    ? 'pending'
    : (outcome ?? (error ? 'error' : 'idle'))
  const errorMessage = error
    ? messageForError(error, 'resendVerification')
    : undefined

  async function sendAgain() {
    setOutcome(null)
    setError(null)
    try {
      const result = await resend.mutateAsync()
      if (result === 'sent' || result === 'unknown') setOutcome(result)
    } catch (reason) {
      setError(reason)
    }
  }

  if (dismissed) return null
  return (
    <EmailVerificationBanner
      email={email}
      status={status}
      errorTitle={errorMessage?.title}
      errorDescription={errorMessage?.description}
      onResend={() => void sendAgain()}
      onDismiss={dismiss}
    />
  )
}
