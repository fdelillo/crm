import { Link } from 'react-router'
export function ForbiddenState() {
  return (
    <section>
      <h1 tabIndex={-1}>No tenés acceso a esta sección</h1>
      <p>Pedile a un administrador si lo necesitás</p>
      <Link to="/">Volver al inicio</Link>
    </section>
  )
}
