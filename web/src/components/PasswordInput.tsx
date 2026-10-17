import { forwardRef, useId, useState } from 'react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

export interface PasswordInputProps extends Omit<
  React.ComponentProps<'input'>,
  'type'
> {
  autoComplete: 'current-password' | 'new-password'
}

export const PasswordInput = forwardRef<HTMLInputElement, PasswordInputProps>(
  function PasswordInput({ autoComplete, id, ...props }, ref) {
    const generatedId = useId()
    const inputId = id ?? generatedId
    const [visible, setVisible] = useState(false)
    return (
      <div className="flex gap-2">
        <Input
          {...props}
          ref={ref}
          id={inputId}
          type={visible ? 'text' : 'password'}
          autoComplete={autoComplete}
        />
        <Button
          type="button"
          variant="outline"
          aria-pressed={visible}
          aria-controls={inputId}
          onClick={() => setVisible((current) => !current)}
        >
          {visible ? 'Ocultar' : 'Mostrar'}
        </Button>
      </div>
    )
  },
)
