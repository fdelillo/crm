import { Link } from 'react-router'
import { useIndustryTemplates } from '@/api/industryTemplates'
import { useTenant } from '@/api/tenant'
import { ErrorState } from '@/components/ErrorState'
import { PageHeader } from '@/components/PageHeader'
import { VerifyToInviteNotice } from '@/components/VerifyToInviteNotice'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { messageForError } from '@/api/errorMessages'
import {
  useCan,
  useInviteBlockedReason,
  useResendEmailVerification,
  useSession,
} from '@/features/auth/session'

const currencyNames = {
  ARS: 'Pesos (ARS)',
  USD: 'Dólares (USD)',
} as const

export function DashboardPage() {
  const sessionQuery = useSession()
  const tenant = useTenant()
  const industryTemplates = useIndustryTemplates()
  const canManageSettings = useCan('settings.manage')
  const blockedReason = useInviteBlockedReason()
  const resendVerification = useResendEmailVerification()
  const resendStatus = resendVerification.isPending
    ? 'pending'
    : resendVerification.isError
      ? 'error'
      : resendVerification.data === 'already_verified'
        ? 'idle'
        : (resendVerification.data ?? 'idle')
  const resendError = resendVerification.error
    ? messageForError(resendVerification.error, 'resendVerification')
    : undefined
  const session = sessionQuery.data
  if (!session) return null

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
      )}
      {canManageSettings && (
        <section
          aria-labelledby="first-steps-heading"
          className="mt-6 space-y-3"
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
            {blockedReason ? (
              <Button
                type="button"
                variant="outline"
                disabled
                aria-describedby="verify-to-invite-notice"
                className="flex min-h-14 h-auto items-center justify-start rounded-lg px-4 py-3 text-left"
              >
                Invitá a tu equipo
              </Button>
            ) : (
              <Link
                className="flex min-h-14 items-center rounded-lg border bg-card px-4 py-3 font-medium text-primary underline underline-offset-4"
                to="/settings/users/invite"
              >
                Invitá a tu equipo
              </Link>
            )}
          </div>
        </section>
      )}
      {canManageSettings && blockedReason && (
        <VerifyToInviteNotice
          id="verify-to-invite-notice"
          email={session.user.email}
          resendStatus={resendStatus}
          errorMessage={resendError}
          onResend={() => {
            if (!resendVerification.isPending) resendVerification.mutate()
          }}
        />
      )}
    </>
  )
}
