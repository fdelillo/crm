import type { paths as Paths001 } from './generated/001'
import type { paths as Paths002 } from './generated/002'

export type paths = Paths001 & Paths002
export type AssertDisjoint<A extends PropertyKey, B extends PropertyKey> =
  Extract<A, B> extends never ? true : never
