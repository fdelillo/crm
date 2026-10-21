import { Button } from './ui/button'
export interface FormAlertProps {
  id?: string
  variant: 'error' | 'warning' | 'info' | 'success'
  title: string
  description?: string
  requestId?: string | null
  announce?: boolean
  action?: { label: string; onClick: () => void; disabled?: boolean }
  secondaryAction?: { label: string; onClick: () => void; disabled?: boolean }
}
const tones = {
  error: 'border-destructive text-destructive',
  warning: 'border-warning bg-warning-muted text-warning',
  info: 'border-primary bg-accent text-accent-foreground',
  success: 'border-success bg-success-muted text-success',
}
export function FormAlert({
  id,
  variant,
  title,
  description,
  requestId,
  announce = true,
  action,
  secondaryAction,
}: FormAlertProps) {
  return (
    <div
      id={id}
      role={announce ? (variant === 'error' ? 'alert' : 'status') : undefined}
      className={`space-y-2 rounded-lg border p-4 ${tones[variant]}`}
    >
      <p className="font-semibold">{title}</p>
      {description && <p>{description}</p>}
      {requestId && <p className="break-all text-sm">Código: {requestId}</p>}
      {(action || secondaryAction) && (
        <div className="flex flex-wrap gap-2">
          {action && (
            <Button
              type="button"
              className="h-11"
              onClick={action.onClick}
              disabled={action.disabled}
            >
              {action.label}
            </Button>
          )}
          {secondaryAction && (
            <Button
              type="button"
              variant="outline"
              className="h-11"
              onClick={secondaryAction.onClick}
              disabled={secondaryAction.disabled}
            >
              {secondaryAction.label}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
