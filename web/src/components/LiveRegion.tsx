import type { ReactNode } from 'react'

export interface LiveRegionProps {
  children?: ReactNode
  className?: string
}

export function LiveRegion({ children, className }: LiveRegionProps) {
  return (
    <div
      role="status"
      aria-atomic="true"
      className={children ? className : 'sr-only'}
    >
      {children}
    </div>
  )
}
