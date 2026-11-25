import { useId } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { SubmitButton } from '@/components/SubmitButton'
import { useOpenerFocus } from '@/lib/useOpenerFocus'

export interface FormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  submitLabel: string
  pendingLabel: string
  // Disables submit and cancel; Escape and a click outside do not close meanwhile.
  pending: boolean
  // With text, submit is disabled and described by this line. Cancel and Escape keep working.
  submitBlockedReason?: string
  onSubmit: React.FormEventHandler<HTMLFormElement>
  children: React.ReactNode
  // Defaults to the first field.
  initialFocusRef?: React.RefObject<HTMLElement | null>
  // Where the focus goes on close when the opener unmounts (the row's [⋮]). Defaults to the opener.
  returnFocusRef?: React.RefObject<HTMLElement | null>
}

// T-002-F103 (002/§16.1, 002/§18, 002/BR-F14): shadcn Dialog with a <form>, controlled from outside.
export function FormDialog({
  open,
  onOpenChange,
  title,
  description,
  submitLabel,
  pendingLabel,
  pending,
  submitBlockedReason,
  onSubmit,
  children,
  initialFocusRef,
  returnFocusRef,
}: FormDialogProps) {
  const reasonId = useId()
  const returnFocus = useOpenerFocus(open, returnFocusRef)
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && pending) return
        onOpenChange(next)
      }}
    >
      {/* The shadcn close button is English-only text; "Cancelar" and Escape are the ways out. */}
      <DialogContent
        showCloseButton={false}
        {...(description ? {} : { 'aria-describedby': undefined })}
        onEscapeKeyDown={(event) => {
          if (pending) event.preventDefault()
        }}
        onInteractOutside={(event) => {
          if (pending) event.preventDefault()
        }}
        onOpenAutoFocus={(event) => {
          const target = initialFocusRef?.current
          if (target) {
            event.preventDefault()
            target.focus()
          }
        }}
        onCloseAutoFocus={returnFocus}
      >
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? (
            <DialogDescription>{description}</DialogDescription>
          ) : null}
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          {children}
          {submitBlockedReason && (
            <p id={reasonId} className="text-sm text-muted-foreground">
              {submitBlockedReason}
            </p>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={() => onOpenChange(false)}
            >
              Cancelar
            </Button>
            <SubmitButton
              pending={pending}
              pendingLabel={pendingLabel}
              disabled={Boolean(submitBlockedReason)}
              describedBy={submitBlockedReason ? reasonId : undefined}
            >
              {submitLabel}
            </SubmitButton>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
