import { useLogoutRequest } from '../api'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ConfirmDialog'

export function LogoutButton() {
  const { logout, confirming, request, confirm, cancel } = useLogoutRequest()
  return (
    <>
      <Button
        type="button"
        variant="outline"
        className="h-11 w-full justify-start border-destructive/30 text-destructive hover:text-destructive"
        disabled={logout.isPending}
        onClick={request}
      >
        {logout.isPending ? 'Cerrando sesión…' : 'Cerrar sesión'}
      </Button>
      <ConfirmDialog
        open={confirming}
        onOpenChange={(open) => {
          if (!open && !logout.isPending) cancel()
        }}
        title="Tenés cambios sin guardar. ¿Salir igual?"
        description="Si salís ahora, se pierden los cambios que todavía no se guardaron."
        confirmLabel="Cerrar sesión"
        pendingLabel="Cerrando sesión…"
        cancelLabel="Quedarme"
        destructive
        pending={logout.isPending}
        onConfirm={confirm}
      />
    </>
  )
}
