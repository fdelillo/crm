const FORMAT = /^(?:[0-9]{11}|[0-9]{2}-[0-9]{8}-[0-9])$/
const WEIGHTS = [5, 4, 3, 2, 7, 6, 5, 4, 3, 2, 1]

// T-F601 (DD-16): same shape and modulo-11 rule as internal/tenant/validation.go.
// The weighted sum including the check digit must be divisible by 11.
export function hasCuitFormat(raw: string): boolean {
  return FORMAT.test(raw)
}

export function isValidCuit(raw: string): boolean {
  if (!hasCuitFormat(raw)) return false
  const digits = raw.replaceAll('-', '')
  const sum = WEIGHTS.reduce(
    (total, weight, index) => total + Number(digits[index]) * weight,
    0,
  )
  return sum % 11 === 0
}
