import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import { AppBrand } from '@/components/AppBrand'
import { FormAlert } from '@/components/FormAlert'
import { PageHeader } from '@/components/PageHeader'
import { SubmitButton } from '@/components/SubmitButton'
import { messageForError } from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import { useResendEmailVerification, useSession } from './session'
import { useConfirmEmailVerification } from './api'
import { useLinkToken } from './useLinkToken'

export function VerifyEmailPage() {
  const token = useLinkToken()
  const sessionQuery = useSession()
  const session = sessionQuery.data
  const confirmEmail = useConfirmEmailVerification()
  const resend = useResendEmailVerification()
  const heading = useRef<HTMLHeadingElement>(null)
  const submitLock = useRef(false)
  const resendLock = useRef(false)
  const invalidLinkFocused = useRef(false)
  const [confirmed, setConfirmed] = useState(false)
  const [invalidLink, setInvalidLink] = useState(false)
  const [confirmError, setConfirmError] = useState<unknown>(null)
  const [resendResult, setResendResult] = useState<
    'sent' | 'already_verified' | 'unknown' | { error: unknown } | null
  >(null)
  const verified = session?.user.email_verified === true

  useEffect(() => {
    if (
      confirmed ||
      resendResult === 'already_verified' ||
      (verified && !token)
    ) {
      heading.current?.focus()
      return
    }
    if (invalidLink && !invalidLinkFocused.current) {
      heading.current?.focus()
      invalidLinkFocused.current = true
    }
  }, [confirmed, invalidLink, resendResult, token, verified])

  async function confirm() {
    if (!token || submitLock.current) return
    submitLock.current = true
    invalidLinkFocused.current = false
    setInvalidLink(false)
    setConfirmError(null)
    try {
      await confirmEmail.mutateAsync({ token })
      setConfirmed(true)
    } catch (error) {
      if (isApiError(error) && error.code === 'token_invalid')
        setInvalidLink(true)
      setConfirmError(error)
    } finally {
      submitLock.current = false
    }
  }

  async function resendLink() {
    if (resendLock.current) return
    resendLock.current = true
    setResendResult(null)
    try {
      const outcome = await resend.mutateAsync()
      setResendResult(outcome)
    } catch (error) {
      setResendResult({ error })
    } finally {
      resendLock.current = false
    }
  }

  const invalidToken = invalidLink
  const confirmMessage = confirmError
    ? messageForError(confirmError, 'verifyEmail')
    : null
  const resendError =
    resendResult && typeof resendResult === 'object' && 'error' in resendResult
      ? resendResult.error
      : null
  const resendMessage = resendError
    ? messageForError(resendError, 'resendVerification')
    : null

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-center px-4 py-8 sm:px-6">
      <div className="mb-6">
        <AppBrand name="CRM" logoSrc={null} />
      </div>
      <section className="space-y-6 rounded-xl border bg-card p-5 shadow-sm sm:p-8">
        <PageHeader title="Confirmá tu email" titleRef={heading} />
        {confirmed ? (
          <>
            <FormAlert
              variant="success"
              title="¡Listo! Tu email quedó confirmado."
            />
            <Link
              className="inline-flex min-h-11 items-center text-primary underline"
              to={session ? '/' : '/login'}
            >
              {session ? 'Ir al inicio' : 'Ingresar'}
            </Link>
          </>
        ) : resendResult === 'already_verified' ||
          (verified && (!token || invalidToken)) ? (
          <>
            <FormAlert
              variant="success"
              title="Tu email ya está confirmado. No hace falta otro enlace."
            />
            <Link
              className="inline-flex min-h-11 items-center text-primary underline"
              to="/"
            >
              Ir al inicio
            </Link>
          </>
        ) : !token ? (
          <>
            <FormAlert variant="warning" title="Este enlace está incompleto." />
            {session ? (
              <button
                type="button"
                className="min-h-11 underline"
                onClick={() => void resendLink()}
                disabled={resend.isPending}
              >
                {resend.isPending
                  ? 'Enviando…'
                  : 'Reenviar email de confirmación'}
              </button>
            ) : (
              <Link
                className="inline-flex min-h-11 items-center underline"
                to="/login"
              >
                Ingresar
              </Link>
            )}
          </>
        ) : invalidToken ? (
          <>
            <FormAlert
              variant="error"
              title={
                confirmMessage?.title ??
                'Este enlace de confirmación ya no sirve.'
              }
              description={confirmMessage?.description}
            />
            {session ? (
              <button
                type="button"
                className="min-h-11 underline"
                onClick={() => void resendLink()}
                disabled={resend.isPending}
              >
                {resend.isPending
                  ? 'Enviando…'
                  : 'Reenviar email de confirmación'}
              </button>
            ) : (
              <>
                <p>Ingresá y pedí uno nuevo desde el aviso.</p>
                <Link
                  className="inline-flex min-h-11 items-center underline"
                  to="/login"
                >
                  Ingresar
                </Link>
              </>
            )}
          </>
        ) : (
          <>
            <p>Tocá el botón para confirmar que este email es tuyo.</p>
            <p>
              Si no creaste una cuenta con este email, no lo confirmes:
              confirmarlo le daría acceso a quien la creó.
            </p>
            {confirmMessage && (
              <FormAlert
                variant="error"
                title={confirmMessage.title}
                description={confirmMessage.description}
                action={
                  confirmMessage.retryable
                    ? { label: 'Reintentar', onClick: () => void confirm() }
                    : undefined
                }
              />
            )}
            <form
              onSubmit={(event) => {
                event.preventDefault()
                void confirm()
              }}
            >
              <SubmitButton
                pending={confirmEmail.isPending}
                pendingLabel="Confirmando…"
                className="h-11 w-full"
              >
                Confirmar mi email
              </SubmitButton>
            </form>
          </>
        )}
        {resendResult === 'sent' && (
          <FormAlert
            variant="success"
            title="Listo, te lo reenviamos. Revisá también el correo no deseado."
          />
        )}
        {resendResult === 'unknown' && (
          <FormAlert
            variant="info"
            title="Si todavía no lo confirmaste, te mandamos un enlace nuevo. Revisá también el correo no deseado."
          />
        )}
        {resendMessage && (
          <FormAlert
            variant="error"
            title={resendMessage.title}
            description={resendMessage.description}
            action={
              resendMessage.retryable
                ? { label: 'Reintentar', onClick: () => void resendLink() }
                : undefined
            }
          />
        )}
      </section>
    </main>
  )
}
