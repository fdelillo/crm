import type { Currency } from '@/api/types'

// T-002-F102 (002/INV-F5, ADR-023, ADR-029): money is always integer cents and is
// converted to and from text by string manipulation, never through a float.
export const MAX_AMOUNT_CENTS = 9007199254740991 // Number.MAX_SAFE_INTEGER
const MAX_AMOUNT_TEXT = String(MAX_AMOUNT_CENTS)

// Es-AR: dot thousands groups (first group 1-3 digits), comma decimals (1-2 digits).
const ES_AR = /^(?<int>\d+|\d{1,3}(?:\.\d{3})+)(?:,(?<dec>\d{1,2}))?$/
// DD-F8: a single dot followed by 1-2 digits and no comma is a decimal point.
const DOT_DECIMAL = /^(?<int>\d+)\.(?<dec>\d{1,2})$/

export type ParsedMoney =
  | { ok: true; amountCents: number }
  | { ok: false; reason: 'empty' | 'format' | 'too_large' }

/** The single place where cents become decimal text: 450050 -> ["4500", "50"]. */
function splitCents(
  amountCents: number,
): [integerPart: string, decimals: string] {
  if (!Number.isSafeInteger(amountCents) || amountCents < 0) {
    throw new RangeError(`invalid amount in cents: ${amountCents}`)
  }
  const digits = String(amountCents).padStart(3, '0')
  return [digits.slice(0, -2), digits.slice(-2)]
}

function groupThousands(integerPart: string): string {
  return integerPart.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}

export function formatMoneyInput(amountCents: number): string {
  const [integerPart, decimals] = splitCents(amountCents)
  return `${groupThousands(integerPart)},${decimals}`
}

export function formatMoney(amountCents: number, currency: Currency): string {
  // Intl accepts a decimal string and formats it exactly, with no float in between.
  return new Intl.NumberFormat('es-AR', {
    style: 'currency',
    currency,
    currencyDisplay: 'code',
  }).format(splitCents(amountCents).join('.') as unknown as number)
}

export function parseMoneyInput(text: string): ParsedMoney {
  const trimmed = text.trim()
  if (trimmed === '') return { ok: false, reason: 'empty' }

  const groups = (DOT_DECIMAL.exec(trimmed) ?? ES_AR.exec(trimmed))?.groups
  if (!groups?.int) return { ok: false, reason: 'format' }
  const integerPart = groups.int.replaceAll('.', '')
  const decimals = groups.dec ?? ''

  const centsText = (integerPart + decimals.padEnd(2, '0')).replace(
    /^0+(?=\d)/,
    '',
  )
  // Fixed-length text comparison: never build a number above 2^53 - 1.
  if (
    centsText.length > MAX_AMOUNT_TEXT.length ||
    (centsText.length === MAX_AMOUNT_TEXT.length && centsText > MAX_AMOUNT_TEXT)
  ) {
    return { ok: false, reason: 'too_large' }
  }
  return { ok: true, amountCents: Number.parseInt(centsText, 10) }
}
