import { messageForError } from '@/api/errorMessages'
import { ErrorState } from '@/components/ErrorState'
import { PageHeader } from '@/components/PageHeader'
import { Skeleton } from '@/components/ui/skeleton'
import { useTenant } from './api'
import { CompanyEditor } from './CompanyEditor'

// T-F602 (US-4, ui.md §13.11): loading and error states; the form lives in CompanyEditor so
// its initial values come from the loaded tenant (INV-F01).
export function CompanyPage() {
  const tenant = useTenant()
  if (tenant.data) return <CompanyEditor tenant={tenant.data} />
  return (
    <>
      <PageHeader
        title="Datos de la empresa"
        description="Se usan en los PDF de tus presupuestos."
        back={{ to: '/settings', label: '← Ajustes' }}
      />
      {tenant.isError ? (
        <ErrorState
          message={{
            ...messageForError(tenant.error, 'tenant'),
            title: 'No pudimos cargar los datos de tu empresa.',
          }}
          onRetry={() => void tenant.refetch()}
          retrying={tenant.isFetching}
        />
      ) : (
        <section
          aria-label="Cargando los datos de la empresa"
          aria-busy="true"
          className="space-y-4"
        >
          <Skeleton aria-hidden="true" className="size-24" />
          {[0, 1, 2, 3, 4, 5].map((item) => (
            <Skeleton key={item} aria-hidden="true" className="h-11 w-full" />
          ))}
        </section>
      )}
    </>
  )
}
