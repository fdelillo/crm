import { useState } from 'react'

const dismissalKey = 'crm.verifyBanner.dismissed'

export function useVerifyBannerDismissed(): [boolean, () => void] {
  const [dismissed, setDismissed] = useState(
    () => sessionStorage.getItem(dismissalKey) === 'true',
  )
  return [
    dismissed,
    () => {
      sessionStorage.setItem(dismissalKey, 'true')
      setDismissed(true)
    },
  ]
}

export function clearVerifyBannerDismissed(): void {
  sessionStorage.removeItem(dismissalKey)
}
