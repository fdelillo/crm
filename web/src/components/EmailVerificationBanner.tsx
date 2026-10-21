import { Button } from '@/components/ui/button'

export type EmailVerificationNoticeStatus =
  'idle' | 'pending' | 'sent' | 'unknown' | 'error'

export interface EmailVerificationBannerProps {
  email: string
  resendStatus: EmailVerificationNoticeStatus
  errorMessage?: { title: string; description?: string }
  onResend: () => void
  onDismiss: () => void
}

export function EmailVerificationBanner({
  email,
  resendStatus,
  errorMessage,
  onResend,
  onDismiss,
}: EmailVerificationBannerProps) {
  const message =
    resendStatus === 'sent'
      ? 'Listo, te lo reenviamos. Revisá también el correo no deseado.'
      : resendStatus === 'unknown'
        ? 'Si todavía no lo confirmaste, te mandamos un enlace nuevo. Revisá también el correo no deseado.'
        : undefined

  return (
    <section
      aria-label="Confirmación de email"
      className="mx-4 my-3 flex flex-wrap items-center gap-3 rounded-lg border border-warning bg-warning-muted p-3 text-warning lg:mx-6"
    >
      <p className="flex-1">
        Confirmá tu email: te mandamos un enlace a {email}.
      </p>
      <Button
        type="button"
        variant="outline"
        className="min-h-11"
        disabled={resendStatus === 'pending'}
        onClick={onResend}
      >
        {resendStatus === 'pending' ? 'Enviando…' : 'Reenviar'}
      </Button>
      <Button
        type="button"
        variant="ghost"
        className="min-h-11"
        aria-label="Ocultar aviso de confirmación de email"
        onClick={onDismiss}
      >
        Ocultar
      </Button>
      {message && (
        <p className="w-full" aria-live="polite">
          {message}
        </p>
      )}
      {resendStatus === 'error' && errorMessage && (
        <div className="w-full" aria-live="polite">
          <p>{errorMessage.title}</p>
          {errorMessage.description && <p>{errorMessage.description}</p>}
        </div>
      )}
    </section>
  )
}
