// T-002-F101 (002/INV-F6): the row that receives focus when `removedId` disappears.
export function focusTargetAfterRemoval(
  orderedIds: readonly string[],
  removedId: string,
): string | null {
  const index = orderedIds.indexOf(removedId)
  if (index === -1) return null
  return orderedIds[index + 1] ?? orderedIds[index - 1] ?? null
}
