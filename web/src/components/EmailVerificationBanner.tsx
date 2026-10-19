import { useState } from 'react'
import { Button } from '@/components/ui/button'

export type EmailVerificationNoticeStatus =
  'idle' | 'pending' | 'sent' | 'unknown' | 'error'

export interface EmailVerificationBannerProps {
  email: string
  status: EmailVerificationNoticeStatus
  errorMessage?: string
  onResend: () => void
  onDismiss: () => void
}

export function EmailVerificationBanner({
  email,
  status,
  errorMessage,
  onResend,
  onDismiss,
}: EmailVerificationBannerProps) {
  const message =
    status === 'sent'
      ? 'Listo, te lo reenviamos. Revisá también el correo no deseado.'
      : status === 'unknown'
        ? 'Si todavía no lo confirmaste, te mandamos un enlace nuevo. Revisá también el correo no deseado.'
        : errorMessage

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
        disabled={status === 'pending'}
        onClick={onResend}
      >
        {status === 'pending' ? 'Enviando…' : 'Reenviar'}
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
    </section>
  )
}

export function useEmailVerificationDismissed(
  email: string,
): [boolean, () => void] {
  const key = `email-verification-dismissed:${email}`
  const [dismissed, setDismissed] = useState(
    () => sessionStorage.getItem(key) === '1',
  )
  return [
    dismissed,
    () => {
      sessionStorage.setItem(key, '1')
      setDismissed(true)
    },
  ]
}
