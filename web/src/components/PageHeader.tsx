import type { ReactNode, Ref } from 'react'
import { Link } from 'react-router'
export interface PageHeaderProps {
  title: string
  description?: string
  back?: { to: string; label: string }
  actions?: ReactNode
  titleRef?: Ref<HTMLHeadingElement>
}
export function PageHeader({
  title,
  description,
  back,
  actions,
  titleRef,
}: PageHeaderProps) {
  return (
    <header className="mb-6 space-y-2">
      {back && (
        <Link
          className="inline-flex min-h-11 items-center text-primary underline"
          to={back.to}
        >
          {back.label}
        </Link>
      )}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <h1 ref={titleRef} tabIndex={-1}>{title}</h1>
        {actions}
      </div>
      {description && <p className="text-muted-foreground">{description}</p>}
    </header>
  )
}
