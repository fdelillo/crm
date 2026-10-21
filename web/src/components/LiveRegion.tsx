import { Children, type ReactNode } from 'react'

export interface LiveRegionProps {
  children?: ReactNode
  className?: string
}

export function LiveRegion({ children, className }: LiveRegionProps) {
  const hasContent = Children.toArray(children).length > 0
  return (
    <div
      role="status"
      aria-atomic="true"
      className={hasContent ? className : 'sr-only'}
    >
      {children}
    </div>
  )
}
