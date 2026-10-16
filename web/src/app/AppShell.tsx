import type { SessionInfo } from '@/api/types'
import { Home, Settings, ChevronDown } from 'lucide-react'
import { NavLink, Outlet } from 'react-router'
import { useSession } from '@/features/auth/session'
import { useTenant, logoUrl } from '@/features/tenant/api'
import { AppBrand } from '@/components/AppBrand'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
} from '@/components/ui/dropdown-menu'
export function AppShell() {
  const { data: session } = useSession()
  return session ? <AuthenticatedShell session={session} /> : null
}
function AuthenticatedShell({ session }: { session: SessionInfo }) {
  const { data: tenant } = useTenant()
  const role = session.user.role === 'admin' ? 'Administrador' : 'Operador'
  return (
    <div className="min-h-dvh lg:pl-60">
      <a
        href="#main"
        className="fixed left-4 top-4 z-[200] -translate-y-24 rounded-lg bg-primary px-4 py-3 text-primary-foreground focus:translate-y-0"
        onClick={() => document.getElementById('main')?.focus()}
      >
        Saltar al contenido
      </a>
      <header className="border-b bg-card px-4 py-3 lg:fixed lg:inset-y-0 lg:left-0 lg:w-60 lg:border-r lg:p-6">
        <AppBrand
          name={tenant?.name ?? session.tenant.name}
          logoSrc={tenant ? logoUrl(tenant) : null}
        />
      </header>
      <nav
        aria-label="Navegación principal"
        className="fixed inset-x-0 bottom-0 z-40 flex justify-around gap-2 border-t bg-card px-4 pb-[env(safe-area-inset-bottom)] lg:inset-x-auto lg:bottom-auto lg:left-0 lg:top-28 lg:w-60 lg:flex-col lg:border-0 lg:p-4"
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
      <div className="hidden lg:fixed lg:bottom-6 lg:left-4 lg:block lg:w-52">
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
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
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
