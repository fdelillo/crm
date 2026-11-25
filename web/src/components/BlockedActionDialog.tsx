import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { useOpenerFocus } from '@/lib/useOpenerFocus'

export interface BlockedActionDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
}

// T-002-F103 (002/§16.1, C-05): says why an action cannot run and what to do first. One button.
// "Entendido" is the Radix cancel part: it is the one that receives the initial focus.
export function BlockedActionDialog({
  open,
  onOpenChange,
  title,
  description,
}: BlockedActionDialogProps) {
  const returnFocus = useOpenerFocus(open)
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent onCloseAutoFocus={returnFocus}>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Entendido</AlertDialogCancel>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
