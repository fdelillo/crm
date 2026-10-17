import { Link } from 'react-router'
import { useIndustryTemplates } from '@/api/industryTemplates'
import { useTenant } from '@/api/tenant'
import { ErrorState } from '@/components/ErrorState'
import { PageHeader } from '@/components/PageHeader'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useCurrentSession } from '@/features/auth/session'

const currencyNames = {
  ARS: 'Pesos (ARS)',
  USD: 'Dólares (USD)',
} as const

export function DashboardPage() {
  const session = useCurrentSession()
  const tenant = useTenant()
  const industryTemplates = useIndustryTemplates()
  const isAdmin = session.user.role === 'admin'

  return (
    <>
      <PageHeader title={`Hola, ${session.user.name}`} />
      {tenant.isPending || industryTemplates.isPending ? (
        <section
          role="status"
          aria-label="Cargando datos de tu empresa"
          aria-busy="true"
          className="space-y-3"
        >
          <Skeleton className="h-24 w-full" />
        </section>
      ) : tenant.isError ? (
        <ErrorState
          message={{
            title: 'No pudimos cargar los datos de tu empresa.',
            retryable: true,
          }}
          onRetry={() => void tenant.refetch()}
          retrying={tenant.isFetching}
        />
      ) : (
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>{tenant.data.name}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <p>
                Rubro:{' '}
                {industryTemplates.data?.items.find(
                  (template) =>
                    template.code === tenant.data.industry_template_code,
                )?.name ?? tenant.data.industry_template_code}
              </p>
              <p>Moneda base: {currencyNames[tenant.data.base_currency]}</p>
            </CardContent>
          </Card>
          {isAdmin && (
            <section
              aria-labelledby="first-steps-heading"
              className="space-y-3"
            >
              <h2 id="first-steps-heading" className="text-lg font-semibold">
                Primeros pasos
              </h2>
              <div className="grid gap-3 sm:grid-cols-2">
                <Link
                  className="flex min-h-14 items-center rounded-lg border bg-card px-4 py-3 font-medium text-primary underline underline-offset-4"
                  to="/settings/company"
                >
                  Completá los datos de tu empresa
                </Link>
                <Link
                  className="flex min-h-14 items-center rounded-lg border bg-card px-4 py-3 font-medium text-primary underline underline-offset-4"
                  to="/settings/users/invite"
                >
                  Invitá a tu equipo
                </Link>
              </div>
            </section>
          )}
        </div>
      )}
    </>
  )
}
