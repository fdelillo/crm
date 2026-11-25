import { useMemo, useRef, useState } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { applyServerFieldErrors, messageForError } from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import type { Stage, StageCreate, StageUpdate } from '@/api/types'
import { FormDialog } from '@/components/FormDialog'
import { FormAlert } from '@/components/FormAlert'
import { useTerminology } from '@/api/labels'
import { stageChanges } from '../pipeline'
import {
  newStageValues,
  stageFormSchema,
  toStageCreate,
  END,
  type StageFormValues,
} from '../schemas'
import { stageTexts } from '../texts'
import { StageForm } from './StageForm'

export interface StageFormDialogProps {
  mode: 'create' | 'edit'
  stages: Stage[]
  stage?: Stage
  // BR-F14: a stage write is pending, so a position cannot be calculated yet.
  writePending: boolean
  returnFocusRef: React.RefObject<HTMLElement | null>
  onClose: () => void
  onCreate: (body: StageCreate) => Promise<void>
  onUpdate: (stage: Stage, changes: StageUpdate) => Promise<void>
  // The server says the stage is no longer there (404): the screen closes this and explains.
  onStageGone: (error: unknown) => void
  // A 422 about a field the form does not have: the list we had was old.
  onListWasOld: () => void
}

const knownFields = ['name', 'type', 'color'] as const

// T-002-F202 (002/§14.2 C-03, §13.1, BR-F3, BR-F12, BR-F14): "Nueva etapa" and "Editar etapa".
export function StageFormDialog({
  mode,
  stages,
  stage,
  writePending,
  returnFocusRef,
  onClose,
  onCreate,
  onUpdate,
  onStageGone,
  onListWasOld,
}: StageFormDialogProps) {
  const terms = useTerminology()
  const texts = useMemo(() => stageTexts(terms), [terms])
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<unknown>(null)
  const submitLock = useRef(false)
  const form = useForm<StageFormValues>({
    resolver: zodResolver(
      stageFormSchema({ mode, stages, stageId: stage?.id, texts }),
    ),
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    defaultValues: stage
      ? { name: stage.name, type: stage.type, color: stage.color, before: END }
      : newStageValues(stages),
  })
  const { setError, setFocus } = form

  async function submit(values: StageFormValues) {
    if (submitLock.current) return
    if (mode === 'edit' && stage) {
      const changes = stageChanges(stage, values)
      // INV-F9: nothing changed, nothing is sent.
      if (Object.keys(changes).length === 0) return onClose()
    }
    submitLock.current = true
    setSubmitting(true)
    setSubmitError(null)
    try {
      if (mode === 'create') await onCreate(toStageCreate(values, stages))
      else if (stage) await onUpdate(stage, stageChanges(stage, values))
    } catch (error) {
      handleError(error)
    } finally {
      submitLock.current = false
      setSubmitting(false)
    }
  }

  function handleError(error: unknown) {
    if (!isApiError(error)) return setSubmitError(error)
    if (error.code === 'unauthenticated') return
    if (error.code === 'not_found' && mode === 'edit') return onStageGone(error)
    if (error.code === 'name_taken') {
      setError('name', {
        type: 'server',
        message: messageForError(error, 'stage').title,
      })
      return setFocus('name')
    }
    if (error.code === 'validation_failed') {
      const unknown = applyServerFieldErrors(
        setError,
        error.fieldErrors,
        knownFields,
        'stage',
      )
      const first = knownFields.find((field) =>
        error.fieldErrors.some((item) => item.field === field),
      )
      if (first) setFocus(first)
      if (error.fieldErrors.length > 0 && unknown.length === 0) return
      onListWasOld()
      return setSubmitError({ listWasOld: true })
    }
    setSubmitError(error)
  }

  const message = !submitError
    ? null
    : typeof submitError === 'object' && 'listWasOld' in submitError
      ? {
          title: 'No pudimos guardar el cambio.',
          description: 'Actualizamos la lista: probá de nuevo.',
          requestId: undefined,
        }
      : messageForError(submitError, 'stage')
  const blockedReason =
    mode === 'create' && writePending && !submitting
      ? 'Esperá a que termine el cambio de orden.'
      : undefined
  return (
    <FormDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
      title={mode === 'create' ? 'Nueva etapa' : 'Editar etapa'}
      submitLabel={mode === 'create' ? 'Agregar etapa' : 'Guardar etapa'}
      pendingLabel={mode === 'create' ? 'Agregando…' : 'Guardando…'}
      pending={submitting}
      submitBlockedReason={blockedReason}
      onSubmit={(event) => void form.handleSubmit(submit)(event)}
      returnFocusRef={returnFocusRef}
    >
      <StageForm
        mode={mode}
        stages={stages}
        typeDescriptions={texts.typeDescriptions}
        form={form}
      />
      {message && (
        <FormAlert
          variant="error"
          title={message.title}
          description={message.description}
          requestId={message.requestId}
        />
      )}
    </FormDialog>
  )
}
