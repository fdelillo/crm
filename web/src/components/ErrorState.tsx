import { Button } from './ui/button'
import { FormAlert } from './FormAlert'
export interface ErrorStateProps {
  message: {
    title: string
    description?: string
    retryable: boolean
    requestId?: string
  }
  onRetry?: () => void
  retrying?: boolean
}
export function ErrorState({
  message,
  onRetry,
  retrying = false,
}: ErrorStateProps) {
  return (
    <div aria-busy={retrying}>
      <FormAlert variant="error" {...message} />
      {onRetry && (
        <Button
          className="mt-3"
          type="button"
          disabled={retrying}
          onClick={onRetry}
        >
          {retrying ? 'Reintentando…' : 'Reintentar'}
        </Button>
      )}
    </div>
  )
}
