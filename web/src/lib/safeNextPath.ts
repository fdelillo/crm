export function safeNextPath(raw: string | null): string {
  if (!raw) return '/'
  let decoded = raw
  try {
    // Nested escapes must not hide a protocol-relative target.
    for (let i = 0; i <= raw.length; i++) {
      if (
        !decoded.startsWith('/') ||
        decoded.startsWith('//') ||
        /[\\\u0000-\u0020]|[a-z][a-z0-9+.-]*:/i.test(decoded)
      )
        return '/'
      const next = decodeURIComponent(decoded)
      if (next === decoded) return raw
      decoded = next
    }
  } catch {
    return '/'
  }
  return '/'
}
