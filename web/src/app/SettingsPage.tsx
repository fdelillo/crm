import { Link } from 'react-router'
import { useSession, useCan } from '@/features/auth/session'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
import { PageHeader } from '@/components/PageHeader'
import { LogoutButton } from '@/features/auth/components/LogoutButton'
export function SettingsPage() {
  const { data: session } = useSession()
  const manage = useCan('settings.manage')
  useDocumentTitle(session ? `Ajustes · ${session.tenant.name}` : 'Ajustes')
  if (!session) return null
  const { user } = session
  return (
    <>
      <PageHeader title="Ajustes" />
      <section
        aria-label="Tu usuario"
        className="mb-6 space-y-1 rounded-lg border bg-card p-4"
      >
        <p className="font-semibold">{user.name}</p>
        <p className="break-all text-muted-foreground">{user.email}</p>
        <p className="text-muted-foreground">
          {user.role === 'admin' ? 'Administrador' : 'Operador'}
        </p>
      </section>
      {manage && (
        <section>
          <h2 className="mb-3 text-lg font-semibold">Tu empresa</h2>
          <div className="divide-y rounded-lg border bg-card">
            {[
              { to: '/settings/company', label: 'Datos de la empresa' },
              { to: '/settings/users', label: 'Usuarios' },
            ].map(({ to, label }) => (
              <Link
                key={to}
                to={to}
                className="flex min-h-14 items-center justify-between px-4 py-3 text-primary"
              >
                {label}
                <span aria-hidden="true">›</span>
              </Link>
            ))}
          </div>
        </section>
      )}
      <div className="mt-6 border-t pt-6">
        <LogoutButton />
      </div>
    </>
  )
}
