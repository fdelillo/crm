import { useLogout } from '../api'
import { Button } from '@/components/ui/button'

export function LogoutButton() {
  const logout = useLogout()
  return (
    <Button
      type="button"
      variant="outline"
      className="h-11 w-full justify-start border-destructive/30 text-destructive hover:text-destructive"
      disabled={logout.isPending}
      onClick={() => logout.mutate()}
    >
      {logout.isPending ? 'Cerrando sesión…' : 'Cerrar sesión'}
    </Button>
  )
}
