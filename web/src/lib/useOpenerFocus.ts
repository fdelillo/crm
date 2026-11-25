import { useCallback, useLayoutEffect, useRef } from 'react'

// T-002-F103 (002/§16.1, INV-F10 de 001): the dialogs of this app are controlled from outside, with
// no Radix trigger, so Radix has nothing to give the focus back to. This remembers the element that
// had the focus when the dialog opened and returns it when the dialog closes. If that element is
// gone by then (a menu item, a row that was removed), it does nothing: whoever removed it decides
// where the focus goes.
export function useOpenerFocus(open: boolean) {
  const opener = useRef<HTMLElement | null>(null)
  useLayoutEffect(() => {
    if (open && document.activeElement instanceof HTMLElement) {
      opener.current = document.activeElement
    }
  }, [open])
  return useCallback((event: Event) => {
    const element = opener.current
    opener.current = null
    event.preventDefault()
    if (element?.isConnected) element.focus()
  }, [])
}
