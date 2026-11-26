import { useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { useTerminology } from '@/api/labels'
import { messageForError } from '@/api/errorMessages'
import { isApiError } from '@/api/errors'
import type { Stage, StageCreate, StageUpdate } from '@/api/types'
import { BlockedActionDialog } from '@/components/BlockedActionDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LiveRegion } from '@/components/LiveRegion'
import { PageHeader } from '@/components/PageHeader'
import { ReadOnlyNotice } from '@/components/ReadOnlyNotice'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useCan, useSession } from '@/features/auth/session'
import { focusTargetAfterRemoval } from '@/lib/removalFocus'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
import {
  useCreateStage,
  useDeleteStage,
  useMoveStage,
  useReloadStages,
  useStages,
  useStageWritePending,
  useUpdateStage,
} from './api'
import { stageTypeLabels } from './labels'
import {
  afterDelete,
  afterMove,
  pipelineViolation,
  type PipelineViolation,
} from './pipeline'
import { stageTexts } from './texts'
import { sameOrder, useListSettled } from './useListSettled'
import { StageFormDialog } from './components/StageFormDialog'
import { StageListItem, type StageAction } from './components/StageListItem'
import { StageMoveDialog } from './components/StageMoveDialog'

type RowOperation = 'update' | 'move' | 'delete'

// One state for the dialog on screen, whatever its kind (the row it came from is `stageId`).
type DialogState =
  | { kind: 'create' }
  | { kind: 'edit'; stageId: string }
  | { kind: 'move'; stageId: string }
  | { kind: 'delete'; stageId: string }
  | { kind: 'blocked'; title: string; description: string }

// 002/§13.1: a failed row action is a toast with the title and, if there is one, the description.
function toastError(error: unknown, reloadStages: () => Promise<void>) {
  if (isApiError(error)) {
    // 401 and 403 are handled by the session layer (INV-F8): it refreshes /me and the screen follows.
    if (error.code === 'unauthenticated' || error.code === 'forbidden') return
    if (error.code === 'validation_failed') {
      void reloadStages()
      return void toast(
        'No pudimos guardar el cambio. Actualizamos la lista: probá de nuevo.',
      )
    }
  }
  const message = messageForError(error, 'stage')
  toast(
    message.description
      ? `${message.title} ${message.description}`
      : message.title,
  )
}

function blockedText(
  violation: PipelineViolation,
  action: 'delete' | 'move',
  stage: Stage,
  stages: Stage[],
  firstNotOpenText: string,
) {
  if (action === 'move')
    return {
      title: `No se puede mover «${stage.name}» ahí`,
      description: firstNotOpenText,
    }
  const title = `No se puede eliminar «${stage.name}»`
  if (violation === 'no_won')
    return {
      title,
      description:
        'Tiene que quedar al menos una etapa de cierre ganado. Antes, marcá otra etapa como «Cierre ganado».',
    }
  const next = afterDelete(stages, stage.id)[0]
  return {
    title,
    description: `Después de eliminarla, la primera etapa sería «${next?.name ?? ''}», que es de cierre. La primera tiene que ser abierta: antes, mové una etapa abierta al principio.`,
  }
}

// T-002-F202 (Historia 1, 002/§14.2): S-13, the stages of the company. Everyone reads; whoever has
// settings.manage also changes (BR-F1). The server decides; hiding the controls is only UX.
export function StagesPage() {
  const terms = useTerminology()
  const texts = useMemo(() => stageTexts(terms), [terms])
  const { data: session } = useSession()
  const canManage = useCan('settings.manage')
  const query = useStages()
  const stages = query.data
  const writePending = useStageWritePending()
  const create = useCreateStage()
  const update = useUpdateStage()
  const move = useMoveStage()
  const remove = useDeleteStage()
  const reloadStages = useReloadStages()
  useDocumentTitle(
    session ? `${texts.title} · ${session.tenant.name}` : texts.title,
  )

  const [dialog, setDialog] = useState<DialogState | null>(null)
  const [rowPending, setRowPending] = useState<Record<string, RowOperation>>({})
  const [announcement, setAnnouncement] = useState('')
  // Where the focus goes when a dialog closes (the [⋮] of the row, or "+ Agregar etapa").
  const returnFocusRef = useRef<HTMLElement | null>(null)
  const addButtonRef = useRef<HTMLButtonElement | null>(null)
  const titleRef = useRef<HTMLHeadingElement | null>(null)
  const triggers = useRef(new Map<string, HTMLButtonElement>())
  const whenListSettled = useListSettled(stages)

  const triggerRefFor = (id: string) => (element: HTMLButtonElement | null) => {
    if (element) triggers.current.set(id, element)
    else triggers.current.delete(id)
  }
  // The focus goes back by id, but never takes it from a control the user already chose (INV-F6).
  const refocus = (id: string | null) => {
    const active = document.activeElement
    if (active && active !== document.body) return
    const target = id
      ? triggers.current.get(id)
      : (addButtonRef.current ?? titleRef.current)
    target?.focus()
  }

  const withRowPending = async <T,>(
    id: string,
    operation: RowOperation,
    run: () => Promise<T>,
  ) => {
    setRowPending((current) => ({ ...current, [id]: operation }))
    try {
      return await run()
    } finally {
      setRowPending((current) =>
        Object.fromEntries(
          Object.entries(current).filter(([key]) => key !== id),
        ),
      )
    }
  }

  const closeDialog = (focusRow?: string) => {
    setDialog(null)
    if (focusRow)
      whenListSettled(
        () => true,
        () => refocus(focusRow),
      )
  }

  async function runMove(stage: Stage, position: number) {
    setAnnouncement(`Moviendo «${stage.name}»…`)
    try {
      const items = await withRowPending(stage.id, 'move', () =>
        move.mutateAsync({ stageId: stage.id, position }),
      )
      const placed = items.findIndex((item) => item.id === stage.id) + 1
      whenListSettled(sameOrder(items), () => {
        setAnnouncement(
          `«${stage.name}» quedó en la posición ${placed} de ${items.length}.`,
        )
        refocus(stage.id)
      })
    } catch (error) {
      setAnnouncement('')
      toastError(error, reloadStages)
      whenListSettled(
        () => true,
        () => refocus(stage.id),
      )
    }
  }

  async function runDelete(stage: Stage) {
    // INV-F6: chosen when the DELETE leaves the queue, over the order the previous writes left.
    const focus: { target: string | null } = { target: null }
    const onSend = (orderedIds: readonly string[]) => {
      focus.target = focusTargetAfterRemoval(orderedIds, stage.id)
    }
    try {
      await withRowPending(stage.id, 'delete', () =>
        remove.mutateAsync({ stageId: stage.id, onSend }),
      )
      toast(`Eliminaste la etapa «${stage.name}».`)
      setDialog(null)
      whenListSettled(
        (ids) => !ids.includes(stage.id),
        () => refocus(focus.target),
      )
    } catch (error) {
      toastError(error, reloadStages)
      closeDialog(stage.id)
    }
  }

  async function createStage(body: StageCreate) {
    setAnnouncement('')
    const created = await create.mutateAsync(body)
    toast(`Agregaste la etapa «${created.name}».`)
    setDialog(null)
  }

  async function updateStage(stage: Stage, changes: StageUpdate) {
    setAnnouncement('')
    const updated = await withRowPending(stage.id, 'update', () =>
      update.mutateAsync({ stageId: stage.id, changes }),
    )
    toast(`Guardamos la etapa «${updated.name}».`)
    setDialog(null)
  }

  function blockMove(stage: Stage, list: Stage[], position: number) {
    const violation = pipelineViolation(afterMove(list, stage.id, position))
    if (!violation) return false
    setDialog({
      kind: 'blocked',
      ...blockedText(violation, 'move', stage, list, texts.firstNotOpen),
    })
    return true
  }

  function onAction(
    stage: Stage,
    action: StageAction,
    trigger: HTMLButtonElement,
  ) {
    if (!stages) return
    returnFocusRef.current = trigger
    const index = stages.findIndex((item) => item.id === stage.id)
    if (action === 'edit') return setDialog({ kind: 'edit', stageId: stage.id })
    if (action === 'delete') {
      const violation = pipelineViolation(afterDelete(stages, stage.id))
      return setDialog(
        violation
          ? {
              kind: 'blocked',
              ...blockedText(
                violation,
                'delete',
                stage,
                stages,
                texts.firstNotOpen,
              ),
            }
          : { kind: 'delete', stageId: stage.id },
      )
    }
    // BR-F14: no position is calculated over a list that a pending write is about to change.
    if (writePending) return
    if (action === 'moveTo')
      return setDialog({ kind: 'move', stageId: stage.id })
    const position = index + 1 + (action === 'moveUp' ? -1 : 1)
    if (position < 1 || position > stages.length) return
    if (!blockMove(stage, stages, position)) void runMove(stage, position)
  }

  const target =
    dialog && 'stageId' in dialog
      ? stages?.find((stage) => stage.id === dialog.stageId)
      : undefined
  const open = canManage ? dialog : null

  return (
    <>
      <PageHeader
        title={texts.title}
        back={{ to: '/settings', label: '← Ajustes' }}
        description={texts.description}
        titleRef={titleRef}
      />
      {!canManage && (
        <div className="mb-4">
          <ReadOnlyNotice text="Solo un administrador puede cambiar las etapas." />
        </div>
      )}
      {canManage && stages && stages.length > 0 && (
        <div className="mb-4">
          <Button
            ref={addButtonRef}
            type="button"
            className="h-11"
            onClick={() => {
              returnFocusRef.current = addButtonRef.current
              setDialog({ kind: 'create' })
            }}
          >
            + Agregar etapa
          </Button>
        </div>
      )}
      {stages === undefined ? (
        query.isError ? (
          <ErrorState
            message={{
              ...messageForError(query.error, 'stage'),
              title: 'No pudimos cargar las etapas.',
            }}
            onRetry={() => void query.refetch()}
            retrying={query.isFetching}
          />
        ) : (
          <section
            aria-label="Cargando las etapas"
            aria-busy="true"
            className="space-y-3"
          >
            {[0, 1, 2, 3].map((item) => (
              <Skeleton key={item} aria-hidden="true" className="h-20 w-full" />
            ))}
          </section>
        )
      ) : stages.length === 0 ? (
        <EmptyState
          title="Todavía no hay etapas."
          action={
            canManage
              ? {
                  label: '+ Agregar etapa',
                  onClick: () => {
                    returnFocusRef.current = null
                    setDialog({ kind: 'create' })
                  },
                }
              : undefined
          }
        />
      ) : (
        <ol className="list-decimal space-y-3 pl-8">
          {stages.map((stage, index) => (
            <StageListItem
              key={stage.id}
              stage={stage}
              isInitial={index === 0}
              typeLabel={stageTypeLabels[stage.type]}
              canManage={canManage}
              isFirst={index === 0}
              isLast={index === stages.length - 1}
              pending={rowPending[stage.id] ?? null}
              movesBlocked={writePending}
              onAction={(action, trigger) => onAction(stage, action, trigger)}
              triggerRef={triggerRefFor(stage.id)}
            />
          ))}
        </ol>
      )}
      <LiveRegion>{announcement || null}</LiveRegion>

      {stages && open?.kind === 'create' && (
        <StageFormDialog
          mode="create"
          stages={stages}
          writePending={writePending}
          returnFocusRef={returnFocusRef}
          onClose={() => setDialog(null)}
          onCreate={createStage}
          onUpdate={updateStage}
          onStageGone={() => undefined}
          onListWasOld={() => void reloadStages()}
        />
      )}
      {stages && target && open?.kind === 'edit' && (
        <StageFormDialog
          mode="edit"
          stages={stages}
          stage={target}
          writePending={writePending}
          returnFocusRef={returnFocusRef}
          onClose={() => setDialog(null)}
          onCreate={createStage}
          onUpdate={updateStage}
          onStageGone={(error) => {
            toastError(error, reloadStages)
            setDialog(null)
          }}
          onListWasOld={() => void reloadStages()}
        />
      )}
      {stages && target && open?.kind === 'move' && (
        <StageMoveDialog
          stage={target}
          stages={stages}
          firstNotOpenText={texts.firstNotOpen}
          returnFocusRef={returnFocusRef}
          onClose={() => setDialog(null)}
          onMove={(position) => {
            setDialog(null)
            void runMove(target, position)
          }}
        />
      )}
      {open?.kind === 'blocked' && (
        <BlockedActionDialog
          open
          onOpenChange={() => setDialog(null)}
          title={open.title}
          description={open.description}
          returnFocusRef={returnFocusRef}
        />
      )}
      {stages && target && open?.kind === 'delete' && (
        <ConfirmDialog
          open
          onOpenChange={(next) => {
            if (!next && rowPending[target.id] !== 'delete')
              closeDialog(target.id)
          }}
          title={`¿Eliminar la etapa «${target.name}»?`}
          description="Sale de la lista y no se puede recuperar. Si después la necesitás, creá una nueva."
          confirmLabel="Eliminar"
          pendingLabel="Eliminando…"
          destructive
          pending={rowPending[target.id] === 'delete'}
          onConfirm={() => void runDelete(target)}
        />
      )}
    </>
  )
}
