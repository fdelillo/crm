import { useOnlineStatus } from '@/lib/useOnlineStatus'
export function OfflineBanner() {
  return useOnlineStatus() ? null : (
    <div
      role="status"
      aria-label="Sin conexión"
      className="bg-warning-muted px-4 py-3 text-warning"
    >
      Sin conexión. Revisá tu internet.
    </div>
  )
}
