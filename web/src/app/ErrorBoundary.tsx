import { Link } from 'react-router'
import { Button } from '@/components/ui/button'

export function ErrorBoundary() {
  return (
    <main>
      <h1>Algo salió mal al mostrar esta pantalla</h1>
      <Button onClick={() => window.location.reload()}>Recargar</Button>
      <Link to="/">Ir al inicio</Link>
    </main>
  )
}
