import { useEffect, useRef } from 'react'
import { Link } from 'react-router'
import { Button } from '@/components/ui/button'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
import { markErrorScreenExit } from './errorScreenExit'

const heading = 'Algo salió mal al mostrar esta pantalla'

// P-F18: this boundary replaces RootLayout, so it sets its own title and focus.
export function ErrorBoundary() {
  const headingRef = useRef<HTMLHeadingElement>(null)
  useDocumentTitle(`${heading} · CRM`)
  useEffect(() => {
    headingRef.current?.focus()
    // Set on mount, not in a cleanup: the remounting RootLayout reads it while
    // rendering, before this component's cleanup would run.
    markErrorScreenExit()
  }, [])
  return (
    <main>
      <h1 ref={headingRef} tabIndex={-1}>
        {heading}
      </h1>
      <Button onClick={() => window.location.reload()}>Recargar</Button>
      <Link to="/">Ir al inicio</Link>
    </main>
  )
}
