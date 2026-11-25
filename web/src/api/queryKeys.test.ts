import { expect, it } from 'vitest'
import { queryKeys } from './queryKeys'

// T-002-F101 (002/§11.3): keys nest under ['settings'] so one invalidation covers a family.
it('builds the settings keys', () => {
  expect(queryKeys.settings.stages).toEqual(['settings', 'stages'])
  expect(queryKeys.settings.catalogItem('x')).toEqual([
    'settings',
    'catalog-items',
    'x',
  ])
  expect(queryKeys.settings.all).toEqual(['settings'])
  expect(queryKeys.settings.labels).toEqual(['settings', 'labels'])
  expect(queryKeys.settings.quoteDefaults).toEqual([
    'settings',
    'quote-defaults',
  ])
  expect(queryKeys.settings.categories).toEqual(['settings', 'categories'])
  expect(queryKeys.settings.catalogItems).toEqual(['settings', 'catalog-items'])
})
