import { useCallback, useLayoutEffect, useRef, useState } from 'react'
import type { Stage } from '@/api/types'

type Waiting = { ready: (ids: readonly string[]) => boolean; run: () => void }

// T-002-F202 (002/INV-F6, §18 "Foco tras mover o quitar"): a write answers before React paints the
// list that answer wrote into the cache. What must happen "once the new list is on screen" (announce
// the result, give the focus back) is queued here and runs in a layout effect, after that paint, as
// soon as the list satisfies `ready`. Moving a node in the DOM can take its focus away: that is
// why the focus is given back by id after painting, not before.
export function useListSettled(stages: readonly Stage[] | undefined) {
  const queue = useRef<Waiting[]>([])
  const [signal, setSignal] = useState(0)
  useLayoutEffect(() => {
    const ids = stages?.map((stage) => stage.id) ?? []
    const still: Waiting[] = []
    for (const waiting of queue.current) {
      if (waiting.ready(ids)) waiting.run()
      else still.push(waiting)
    }
    queue.current = still
  }, [stages, signal])
  return useCallback((ready: Waiting['ready'], run: Waiting['run']) => {
    queue.current.push({ ready, run })
    setSignal((count) => count + 1)
  }, [])
}

export const sameOrder =
  (expected: readonly Pick<Stage, 'id'>[]) => (ids: readonly string[]) =>
    ids.length === expected.length &&
    ids.every((id, index) => id === expected[index]?.id)
