import { Link } from 'react-router'
import { Button } from './ui/button'

type EmptyStateAction =
  | {
      label: string
      to: string
      onClick?: never
      disabled?: boolean
      ariaDescribedBy?: string
    }
  | {
      label: string
      to?: never
      onClick: () => void
      disabled?: boolean
      ariaDescribedBy?: string
    }

export interface EmptyStateProps {
  title: string
  description?: string
  action?: EmptyStateAction
}
export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <section className="space-y-3 rounded-lg border bg-card p-6">
      <h2 className="text-lg font-semibold">{title}</h2>
      {description && <p className="text-muted-foreground">{description}</p>}
      {action &&
        (action.to && !action.disabled ? (
          <Button asChild>
            <Link to={action.to}>{action.label}</Link>
          </Button>
        ) : (
          <Button
            onClick={action.onClick}
            disabled={action.disabled}
            aria-describedby={action.ariaDescribedBy}
          >
            {action.label}
          </Button>
        ))}
    </section>
  )
}
