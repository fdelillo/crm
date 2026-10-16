import { Link } from 'react-router'
import { Button } from './ui/button'
export interface EmptyStateProps {
  title: string
  description?: string
  action?:
    { label: string; to: string } | { label: string; onClick: () => void }
}
export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <section className="space-y-3 rounded-lg border bg-card p-6">
      <h2 className="text-lg font-semibold">{title}</h2>
      {description && <p className="text-muted-foreground">{description}</p>}
      {action &&
        ('to' in action ? (
          <Button asChild>
            <Link to={action.to}>{action.label}</Link>
          </Button>
        ) : (
          <Button onClick={action.onClick}>{action.label}</Button>
        ))}
    </section>
  )
}
