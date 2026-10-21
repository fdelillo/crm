import { FormAlert } from './FormAlert'
import { LiveRegion } from './LiveRegion'
import { Button } from './ui/button'
import type { EmailVerificationNoticeStatus } from './EmailVerificationBanner'

export interface VerifyToInviteNoticeProps {
  id: string
  email: string
  description?: string
  resendStatus: EmailVerificationNoticeStatus
  errorMessage?: { title: string; description?: string; retryable: boolean }
  onResend: () => void
}

export function VerifyToInviteNotice({
  id,
  email,
  description,
  resendStatus,
  errorMessage,
  onResend,
}: VerifyToInviteNoticeProps) {
  return (
    <div className="space-y-2">
      <FormAlert
        id={id}
        variant="warning"
        announce={false}
        title="Confirmá tu email para invitar a tu equipo."
        description={
          description ??
          `Te mandamos un enlace a ${email}. Si no lo encontrás, pedí uno nuevo.`
        }
        action={{
          label:
            resendStatus === 'pending'
              ? 'Enviando…'
              : 'Reenviar email de confirmación',
          onClick: onResend,
          disabled: resendStatus === 'pending',
        }}
      />
      <LiveRegion>
        {resendStatus === 'sent' ? (
          <p>Listo, te lo reenviamos. Revisá también el correo no deseado.</p>
        ) : resendStatus === 'unknown' ? (
          <p>
            Si todavía no lo confirmaste, te mandamos un enlace nuevo. Revisá
            también el correo no deseado.
          </p>
        ) : resendStatus === 'error' && errorMessage ? (
          <div>
            <p className="font-semibold">{errorMessage.title}</p>
            {errorMessage.description && <p>{errorMessage.description}</p>}
            {errorMessage.retryable && (
              <Button type="button" className="mt-2 h-11" onClick={onResend}>
                Reintentar
              </Button>
            )}
          </div>
        ) : null}
      </LiveRegion>
    </div>
  )
}
