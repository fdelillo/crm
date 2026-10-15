import type { paths as Paths001 } from './generated/001'

export type paths = Paths001
export type AssertDisjoint<A extends PropertyKey, B extends PropertyKey> =
  Extract<A, B> extends never ? true : never
