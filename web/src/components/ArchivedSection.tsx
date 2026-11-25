import { useId, useState } from 'react'
import { ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'

export interface ArchivedSectionProps {
  label: string
  count: number
  children: React.ReactNode
}

// T-002-F103 (002/§16.1, 002/§18): disclosure. The content follows the button in reading order.
export function ArchivedSection({
  label,
  count,
  children,
}: ArchivedSectionProps) {
  const [open, setOpen] = useState(false)
  const regionId = useId()
  if (count === 0) return null
  return (
    <div>
      <Button
        type="button"
        variant="ghost"
        aria-expanded={open}
        aria-controls={regionId}
        onClick={() => setOpen((current) => !current)}
      >
        <ChevronRight
          aria-hidden="true"
          className={open ? 'rotate-90' : undefined}
        />
        {label} ({count})
      </Button>
      <div id={regionId} hidden={!open} className="mt-2">
        {children}
      </div>
    </div>
  )
}
