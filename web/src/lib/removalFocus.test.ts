import { expect, it } from 'vitest'
import { focusTargetAfterRemoval } from './removalFocus'

// T-002-F101 (002/INV-F6): focus goes to the next row, else the previous one, else nowhere.
it.each([
  [['a', 'b', 'c'], 'b', 'c'],
  [['a', 'b', 'c'], 'c', 'b'],
  [['a'], 'a', null],
  [['a', 'b'], 'zzz', null],
  [[], 'a', null],
])('%j without %s focuses %s', (ids, removed, target) => {
  expect(focusTargetAfterRemoval(ids, removed)).toBe(target)
})
