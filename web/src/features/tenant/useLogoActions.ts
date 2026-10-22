import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import {
  fieldErrorMessage,
  logoRejectMessage,
  messageForError,
  type UserMessage,
} from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import { useDeleteLogo, useUploadLogo } from './api'
import { prepareLogo, type PrepareLogoResult } from './logo/prepareLogo'

export type LogoPending = 'processing' | 'upload' | 'remove' | null

function describeFailure(error: unknown): UserMessage | null {
  if (isApiError(error)) {
    // A 401 is handled by the global session layer (§12.4); the login explains it.
    if (error.code === 'unauthenticated') return null
    const fileError = error.fieldErrors.find((item) => item.field === 'file')
    if (error.code === 'validation_failed' && fileError)
      return {
        title: fieldErrorMessage('file', fileError.code),
        retryable: false,
      }
  }
  return messageForError(error, 'logo')
}

/**
 * T-F603 (DD-F26, DD-F32, BR-F10, ui.md §13.11, §14.4): prepares, uploads and removes the logo.
 * `pending` is also what keeps the unsaved-changes guard on while the logo is in flight.
 */
export function useLogoActions() {
  const upload = useUploadLogo()
  const remove = useDeleteLogo()
  const [pending, setPending] = useState<LogoPending>(null)
  const [error, setError] = useState<UserMessage | null>(null)
  const [confirmingRemoval, setConfirmingRemoval] = useState(false)
  // Where the focus goes once the confirmation closes (INV-F10): the dialog has no trigger element.
  const [focusTarget, setFocusTarget] = useState<'picker' | 'remove' | null>(
    null,
  )
  const retryAction = useRef<(() => void) | null>(null)
  const busy = useRef(false)
  const mounted = useRef(false)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  async function send(prepared: Extract<PrepareLogoResult, { ok: true }>) {
    busy.current = true
    retryAction.current = () => void send(prepared)
    setError(null)
    setPending('upload')
    try {
      await upload.mutateAsync(prepared.file)
      toast(
        prepared.resized
          ? 'Logo actualizado. Lo achicamos para que pese menos.'
          : 'Logo actualizado.',
      )
    } catch (failure) {
      setError(describeFailure(failure))
    } finally {
      busy.current = false
      setPending(null)
    }
  }

  async function selectFile(file: File) {
    if (busy.current) return
    busy.current = true
    setError(null)
    setPending('processing')
    const prepared = await prepareLogo(file)
    // The page may be gone (session expired, or the user chose to leave): do not upload then.
    if (!mounted.current) return
    if (!prepared.ok) {
      busy.current = false
      setPending(null)
      setError({ title: logoRejectMessage(prepared.reason), retryable: false })
      return
    }
    await send(prepared)
  }

  async function confirmRemoval() {
    if (busy.current) return
    busy.current = true
    retryAction.current = () => void confirmRemoval()
    setError(null)
    setPending('remove')
    try {
      await remove.mutateAsync()
      toast('Quitamos el logo.')
      setFocusTarget('picker')
    } catch (failure) {
      setError(describeFailure(failure))
      setFocusTarget('remove')
    } finally {
      busy.current = false
      setPending(null)
      setConfirmingRemoval(false)
    }
  }

  return {
    pending,
    error,
    selectFile: (file: File) => void selectFile(file),
    retry: () => retryAction.current?.(),
    confirmingRemoval,
    askToRemove: () => setConfirmingRemoval(true),
    cancelRemoval: () => {
      setConfirmingRemoval(false)
      setFocusTarget('remove')
    },
    focusTarget,
    focusHandled: () => setFocusTarget(null),
    confirmRemoval: () => void confirmRemoval(),
  }
}
