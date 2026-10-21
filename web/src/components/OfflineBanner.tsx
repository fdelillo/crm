import { useOnlineStatus } from '@/lib/useOnlineStatus'
import { LiveRegion } from './LiveRegion'

export function OfflineBanner() {
  const online = useOnlineStatus()
  return (
    <LiveRegion>
      {!online && (
        <div className="bg-warning-muted px-4 py-3 text-warning">
          Sin conexión. Revisá tu internet.
        </div>
      )}
    </LiveRegion>
  )
}
