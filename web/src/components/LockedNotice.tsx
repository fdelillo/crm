import { Button } from './ui/button'

export interface LockedNoticeProps {
  title: string
  description: string
  onResetPassword: () => void
  onRetry: () => void
}

export function LockedNotice({
  title,
  description,
  onResetPassword,
  onRetry,
}: LockedNoticeProps) {
  return (
    <section
      aria-labelledby="locked-title"
      className="space-y-4 rounded-lg border p-4"
    >
      <div className="space-y-2">
        <h2 id="locked-title" className="font-semibold">
          {title}
        </h2>
        <p>{description}</p>
      </div>
      <div className="flex flex-wrap gap-2">
        <Button type="button" className="h-11" onClick={onResetPassword}>
          Restablecer contraseña
        </Button>
        <Button
          type="button"
          variant="outline"
          className="h-11"
          onClick={onRetry}
        >
          Volver a intentar
        </Button>
      </div>
    </section>
  )
}
