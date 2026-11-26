import {
  useIsMutating,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query'
import { api } from '@/api/client'
import { isApiError, unwrap, type ApiError } from '@/api/errors'
import { queryKeys } from '@/api/queryKeys'
import type { Stage, StageCreate, StageUpdate } from '@/api/types'

// T-002-F202 (002/INV-F3, DD-F10): the four stage writes share one queue, so each leaves after the
// previous one answered. Moving and creating change the positions of other stages: a response that
// left earlier must not be written over a list that a later one already brought.
export const STAGE_CHANGES_SCOPE = { id: 'stage-changes' }

// T-002-F202 (002/INV-F11): the four mutations carry this key; it is what BR-F14 counts.
export const STAGE_MUTATION_KEY = ['settings', 'stages', 'write'] as const

/**
 * True while any stage write is pending: on the wire or waiting in the queue of the scope (S-F7:
 * TanStack Query marks a queued mutation `pending` from `mutate`). Decides BR-F14 in one place.
 */
export function useStageWritePending(): boolean {
  return useIsMutating({ mutationKey: STAGE_MUTATION_KEY }) > 0
}

export function useStages() {
  return useQuery<Stage[], ApiError>({
    queryKey: queryKeys.settings.stages,
    queryFn: async ({ signal }) =>
      unwrap(await api.GET('/stages', { signal })).items,
  })
}

// INV-F3: a read that left before this response and answers after it would bring the old list back.
async function writeStages(
  client: QueryClient,
  update: (stages: Stage[] | undefined) => Stage[] | undefined,
) {
  await client.cancelQueries({
    queryKey: queryKeys.settings.stages,
    exact: true,
  })
  client.setQueryData<Stage[]>(queryKeys.settings.stages, update)
}

// The mutation stays pending until the list on screen is the one this write confirmed (INV-F11),
// so the read that follows is awaited here and not left running.
const reloadStages = (client: QueryClient) =>
  client.invalidateQueries({ queryKey: queryKeys.settings.stages, exact: true })

// 002/§11.3: a 404, an invalid_state or a name_taken says the list we had was old.
async function reloadWhenListWasOld(client: QueryClient, error: unknown) {
  if (!isApiError(error)) return
  if (['not_found', 'invalid_state', 'name_taken'].includes(error.code ?? ''))
    await reloadStages(client)
}

export function useCreateStage() {
  const client = useQueryClient()
  return useMutation<Stage, ApiError, StageCreate>({
    scope: STAGE_CHANGES_SCOPE,
    mutationKey: STAGE_MUTATION_KEY,
    mutationFn: async (body) => unwrap(await api.POST('/stages', { body })),
    // The response has only the new stage; the positions of the others changed.
    onSuccess: () => reloadStages(client),
    onError: (error) => reloadWhenListWasOld(client, error),
  })
}

export function useUpdateStage() {
  const client = useQueryClient()
  return useMutation<
    Stage,
    ApiError,
    { stageId: string; changes: StageUpdate }
  >({
    scope: STAGE_CHANGES_SCOPE,
    mutationKey: STAGE_MUTATION_KEY,
    mutationFn: async ({ stageId, changes }) =>
      unwrap(
        await api.PATCH('/stages/{stageId}', {
          params: { path: { stageId } },
          body: changes,
        }),
      ),
    // An update changes no position: the response replaces its own stage.
    onSuccess: (stage) =>
      writeStages(client, (stages) =>
        stages?.map((item) => (item.id === stage.id ? stage : item)),
      ),
    onError: (error) => reloadWhenListWasOld(client, error),
  })
}

export function useMoveStage() {
  const client = useQueryClient()
  return useMutation<Stage[], ApiError, { stageId: string; position: number }>({
    scope: STAGE_CHANGES_SCOPE,
    mutationKey: STAGE_MUTATION_KEY,
    mutationFn: async ({ stageId, position }) =>
      unwrap(
        await api.POST('/stages/{stageId}/move', {
          params: { path: { stageId } },
          body: { position },
        }),
      ).items,
    onSuccess: (items) => writeStages(client, () => items),
    onError: (error) => reloadWhenListWasOld(client, error),
  })
}

export function useDeleteStage() {
  const client = useQueryClient()
  return useMutation<
    void,
    ApiError,
    {
      stageId: string
      // Called when the DELETE leaves the queue, with the ids in the order the last confirmed write
      // left (INV-F6): a write that was queued before it may have moved the neighbours.
      onSend?: (orderedIds: readonly string[]) => void
    }
  >({
    scope: STAGE_CHANGES_SCOPE,
    mutationKey: STAGE_MUTATION_KEY,
    mutationFn: async ({ stageId, onSend }) => {
      onSend?.(
        client
          .getQueryData<Stage[]>(queryKeys.settings.stages)
          ?.map((stage) => stage.id) ?? [],
      )
      return unwrap<void>(
        await api.DELETE('/stages/{stageId}', {
          params: { path: { stageId } },
        }),
      )
    },
    onSuccess: () => reloadStages(client),
    onError: (error) => reloadWhenListWasOld(client, error),
  })
}

/** Reads the list again after a write found out that the one on screen was old. */
export function useReloadStages() {
  const client = useQueryClient()
  return () => reloadStages(client)
}
