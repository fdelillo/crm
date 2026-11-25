import { expect, it } from 'vitest'
import { countCodePoints } from './textLength'

// T-002-F101 (002/INV-F10, NFR-F5): lengths count code points, as the server does.
it.each([
  ['Obra', 4],
  ['ñ'.repeat(60), 60],
  ['👷', 1],
  ['', 0],
])('counts %j as %i', (text, count) => {
  expect(countCodePoints(text)).toBe(count)
})
