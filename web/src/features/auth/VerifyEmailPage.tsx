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
  const [confirmed, setConfirmed] = useState(false)
  const [invalidLink, setInvalidLink] = useState(false)
  const [submitError, setSubmitError] = useState<unknown>(null)
  const verified = session?.user.email_verified === true

  useEffect(() => {
    if (verified && !token) heading.current?.focus()
  }, [token, verified])

  async function confirm() {
    if (!token || submitLock.current) return
    submitLock.current = true
    setSubmitError(null)
    try {
      await confirmEmail.mutateAsync({ token })
      setConfirmed(true)
    } catch (error) {
      if (isApiError(error) && error.code === 'token_invalid')
        setInvalidLink(true)
      setSubmitError(error)
    } finally {
      submitLock.current = false
    }
  }

  async function resendLink() {
    try {
      const outcome = await resend.mutateAsync()
      if (outcome === 'sent') setSubmitError({ resendOutcome: outcome })
      if (outcome === 'unknown') setSubmitError({ resendOutcome: outcome })
    } catch (error) {
      setSubmitError(error)
    }
  }

  const invalidToken =
    invalidLink ||
    (isApiError(submitError) && submitError.code === 'token_invalid')
  const message = invalidToken
    ? messageForError(submitError, 'verifyEmail')
    : submitError && !('resendOutcome' in (submitError as object))
      ? messageForError(submitError, 'resendVerification')
      : null
  const resendOutcome =
    submitError &&
    typeof submitError === 'object' &&
    'resendOutcome' in submitError
      ? (submitError as { resendOutcome: string }).resendOutcome
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
        ) : verified ? (
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
              >
                Reenviar email de confirmación
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
                message?.title ?? 'Este enlace de confirmación ya no sirve.'
              }
              description={message?.description}
            />
            {session ? (
              <button
                type="button"
                className="min-h-11 underline"
                onClick={() => void resendLink()}
                disabled={resend.isPending}
              >
                Reenviar email de confirmación
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
            {message && (
              <FormAlert
                variant="error"
                title={message.title}
                description={message.description}
                action={
                  message.retryable
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
        {resendOutcome === 'sent' && (
          <FormAlert
            variant="success"
            title="Listo, te lo reenviamos. Revisá también el correo no deseado."
          />
        )}
        {resendOutcome === 'unknown' && (
          <FormAlert
            variant="info"
            title="Si todavía no lo confirmaste, te mandamos un enlace nuevo. Revisá también el correo no deseado."
          />
        )}
        {message && !invalidToken && !resendOutcome && (
          <FormAlert
            variant="error"
            title={message.title}
            description={message.description}
            action={
              message.retryable
                ? { label: 'Reintentar', onClick: () => void resendLink() }
                : undefined
            }
          />
        )}
      </section>
    </main>
  )
}
