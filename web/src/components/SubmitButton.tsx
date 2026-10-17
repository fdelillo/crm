import { LoaderCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'

export interface SubmitButtonProps {
  pending: boolean
  children: React.ReactNode
  pendingLabel: string
  disabled?: boolean
  variant?: 'default' | 'destructive'
  className?: string
}

export function SubmitButton({
  pending,
  children,
  pendingLabel,
  disabled = false,
  variant = 'default',
  className,
}: SubmitButtonProps) {
  return (
    <Button
      type="submit"
      variant={variant}
      disabled={pending || disabled}
      className={className}
    >
      {pending && (
        <LoaderCircle
          aria-hidden="true"
          className="size-4 animate-spin motion-reduce:animate-none"
        />
      )}
      {pending ? pendingLabel : children}
    </Button>
  )
}
