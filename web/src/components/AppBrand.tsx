import { useState } from 'react'
import { companyInitials } from '@/lib/companyInitials'
export interface AppBrandProps {
  name: string
  logoSrc: string | null
}
export function AppBrand({ name, logoSrc }: AppBrandProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null)
  const initials = companyInitials(name)
  return (
    <div className="flex min-w-0 items-center gap-3">
      {logoSrc && logoSrc !== failedSrc ? (
        <img
          className="size-11 shrink-0 rounded-lg object-contain"
          src={logoSrc}
          alt={`Logo de ${name}`}
          onError={() => setFailedSrc(logoSrc)}
        />
      ) : (
        <span
          aria-hidden="true"
          className="flex size-11 shrink-0 items-center justify-center rounded-lg bg-accent font-semibold text-accent-foreground"
        >
          {initials}
        </span>
      )}
      <span className="min-w-0 break-words font-semibold">{name}</span>
    </div>
  )
}
