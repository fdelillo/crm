import { useLocation } from 'react-router'
import type { SessionExpiredLoginState } from '@/lib/sessionExpiredRedirect'
export function LoginPage() {
  const location = useLocation()
  const expired =
    new URLSearchParams(location.search).get('reason') === 'session_expired'
  const state = location.state as SessionExpiredLoginState | null
  return (
    <>
      <h1 tabIndex={-1}>Ingresar</h1>
      {expired && (
        <div role="status">
          <p>Tu sesión se cerró. Ingresá de nuevo para seguir.</p>
          <p>
            Las sesiones se cierran solas después de 24 horas sin uso o a los 7
            días.
          </p>
          {state?.unsavedChangesLost && (
            <p>Los cambios que no habías guardado se perdieron.</p>
          )}
        </div>
      )}
    </>
  )
}
export function SignupPage() {
  return <h1>Registro</h1>
}
export function ForgotPasswordPage() {
  return <h1>Olvidé mi contraseña</h1>
}
export function ResetPasswordPage() {
  return <h1>Restablecer contraseña</h1>
}
export function VerifyEmailPage() {
  return <h1>Confirmá tu email</h1>
}
export function AcceptInvitationPage() {
  return <h1>Aceptar invitación</h1>
}
