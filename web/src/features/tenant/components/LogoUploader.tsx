import { useState } from 'react'
import { Info } from 'lucide-react'
import type { UserMessage } from '@/api/errorMessages'
import { FormAlert } from '@/components/FormAlert'
import { LiveRegion } from '@/components/LiveRegion'
import { Button, buttonVariants } from '@/components/ui/button'
import { companyInitials } from '@/lib/companyInitials'
import { cn } from '@/lib/utils'

export const LOGO_INPUT_ID = 'company-logo-input'
export const LOGO_REMOVE_ID = 'company-logo-remove'

const pendingText = {
  processing: 'Preparando la imagen…',
  upload: 'Subiendo logo…',
  remove: 'Quitando logo…',
} as const

export interface LogoUploaderProps {
  companyName: string
  /** logoUrl(tenant); null when has_logo is false. */
  logoSrc: string | null
  pending: keyof typeof pendingText | null
  error: UserMessage | null
  onSelectFile: (file: File) => void
  onRemove: () => void
  /** Shown as "Reintentar" on a retryable error. */
  onRetry?: () => void
}

// T-F603 (DD-F11, DD-F12, INV-F19, ui.md §13.11, §15.2).
export function LogoUploader({
  companyName,
  logoSrc,
  pending,
  error,
  onSelectFile,
  onRemove,
  onRetry,
}: LogoUploaderProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null)
  const busy = pending !== null
  const showImage = logoSrc !== null && logoSrc !== failedSrc
  return (
    <section aria-labelledby="company-logo-heading" className="space-y-3">
      <h2 id="company-logo-heading" className="text-lg font-semibold">
        Logo
      </h2>
      <div className="flex items-center gap-4">
        {showImage ? (
          <img
            className="size-24 shrink-0 rounded-lg border object-contain"
            src={logoSrc}
            alt={`Logo de ${companyName}`}
            onError={() => setFailedSrc(logoSrc)}
          />
        ) : (
          <span
            aria-hidden="true"
            className="flex size-24 shrink-0 items-center justify-center rounded-lg bg-accent text-2xl font-semibold text-accent-foreground"
          >
            {companyInitials(companyName)}
          </span>
        )}
        <div className="space-y-1">
          {!logoSrc && (
            <p className="font-medium">Todavía no subiste un logo</p>
          )}
          <p id="company-logo-help" className="text-sm text-muted-foreground">
            PNG o JPG. Si la imagen es muy grande, la achicamos.
          </p>
        </div>
      </div>
      <div className="flex flex-wrap gap-3">
        <label
          className={cn(
            buttonVariants({ variant: 'outline' }),
            'h-11 cursor-pointer has-[:disabled]:pointer-events-none has-[:disabled]:opacity-50 has-[:focus-visible]:ring-[3px] has-[:focus-visible]:ring-ring/50',
          )}
        >
          Cambiar logo
          <input
            id={LOGO_INPUT_ID}
            type="file"
            accept="image/png,image/jpeg"
            className="sr-only"
            disabled={busy}
            aria-describedby="company-logo-help"
            onChange={(event) => {
              const file = event.currentTarget.files?.[0]
              // Clearing lets the same file be chosen again.
              event.currentTarget.value = ''
              if (file) onSelectFile(file)
            }}
          />
        </label>
        <Button
          id={LOGO_REMOVE_ID}
          type="button"
          variant="outline"
          className="h-11"
          disabled={busy || !logoSrc}
          onClick={onRemove}
        >
          Quitar
        </Button>
      </div>
      <LiveRegion className="text-sm text-muted-foreground">
        {pending && (
          <p className="flex items-center gap-2">
            <Info aria-hidden="true" className="size-4" />
            {pendingText[pending]}
          </p>
        )}
      </LiveRegion>
      {error && (
        <FormAlert
          variant="error"
          title={error.title}
          description={error.description}
          requestId={error.requestId}
          action={
            error.retryable && onRetry
              ? { label: 'Reintentar', onClick: onRetry }
              : undefined
          }
        />
      )}
    </section>
  )
}
