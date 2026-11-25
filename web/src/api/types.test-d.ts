import { expectTypeOf, test } from 'vitest'
import type { components } from './generated/001'
import type { CatalogItem, Labels, Money, StageType } from './types'

// T-002-F101 (002/§12.2): the catalog price is the Money of the 001 contract.
test('derives the 002 types from the generated contract', () => {
  expectTypeOf<Money>().toEqualTypeOf<components['schemas']['Money']>()
  expectTypeOf<CatalogItem['reference_price']>().toEqualTypeOf<Money | null>()
  expectTypeOf<StageType>().toEqualTypeOf<'open' | 'won' | 'lost'>()
  expectTypeOf<Labels['project']['gender']>().toEqualTypeOf<
    'masculine' | 'feminine'
  >()
})
