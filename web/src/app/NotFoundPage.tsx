import { Link } from 'react-router'
import { PageHeader } from '@/components/PageHeader'
export function NotFoundPage() {
  return (
    <main id="main" tabIndex={-1} className="mx-auto max-w-3xl p-6">
      <PageHeader title="No encontramos esta página." />
      <Link className="text-primary underline" to="/">
        Ir al inicio
      </Link>
    </main>
  )
}
