import { Button } from './ui/button'
export interface FormAlertProps {
  variant: 'error' | 'warning' | 'info' | 'success'
  title: string
  description?: string
  requestId?: string | null
  action?: { label: string; onClick: () => void }
  secondaryAction?: { label: string; onClick: () => void }
}
const tones = {
  error: 'border-destructive text-destructive',
  warning: 'border-warning bg-warning-muted text-warning',
  info: 'border-primary bg-accent text-accent-foreground',
  success: 'border-success bg-success-muted text-success',
}
export function FormAlert({
  variant,
  title,
  description,
  requestId,
  action,
  secondaryAction,
}: FormAlertProps) {
  return (
    <div
      role={variant === 'error' ? 'alert' : 'status'}
      className={`space-y-2 rounded-lg border p-4 ${tones[variant]}`}
    >
      <p className="font-semibold">{title}</p>
      {description && <p>{description}</p>}
      {requestId && <p className="break-all text-sm">Código: {requestId}</p>}
      {(action || secondaryAction) && (
        <div className="flex flex-wrap gap-2">
          {action && (
            <Button type="button" onClick={action.onClick}>
              {action.label}
            </Button>
          )}
          {secondaryAction && (
            <Button
              type="button"
              variant="outline"
              onClick={secondaryAction.onClick}
            >
              {secondaryAction.label}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
