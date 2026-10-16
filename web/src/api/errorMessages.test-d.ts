import { expectTypeOf, test } from 'vitest'
import { errorMessages, type UserMessage } from './errorMessages'
import type { ErrorCode } from './types'
// T-F101 (INV-F05): adding a code without a message must fail compilation.
test('requires a message for every contract error code', () => {
  expectTypeOf(errorMessages).toExtend<Record<ErrorCode, UserMessage>>()
  // @ts-expect-error A new contract code requires its own message.
  const expanded: Record<ErrorCode | 'new_contract_code', UserMessage> =
    errorMessages
  void expanded
})
