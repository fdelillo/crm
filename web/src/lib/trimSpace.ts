// T-002-F201 (002/INV-F9; revisión 1 de Codex del PR #78): the server trims text with Go's
// strings.TrimSpace, whose spaces are unicode.IsSpace. String.prototype.trim differs in two
// characters: it keeps U+0085 (Go removes it) and removes U+FEFF (Go keeps it). Text the client
// compares or sends trimmed goes through this so both sides agree on what changed and on empty.
const goSpace =
  '\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000'
const edges = new RegExp(`^[${goSpace}]+|[${goSpace}]+$`, 'g')

export function trimSpace(text: string): string {
  return text.replace(edges, '')
}
