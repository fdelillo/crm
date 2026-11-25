import { describe, expect, it } from 'vitest'
import { trimSpace } from './trimSpace'

// T-002-F201 (002/INV-F9; revisión 1 de Codex del PR #78): the server trims names with Go's
// strings.TrimSpace (unicode.IsSpace), which differs from String.prototype.trim in two characters.
describe('trimSpace', () => {
  const goSpaces = [
    '\t',
    '\n',
    '\v',
    '\f',
    '\r',
    ' ',
    '\u0085',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    ' ',
    '　',
  ]

  it.each(goSpaces.map((space) => [space.codePointAt(0)!.toString(16), space]))(
    'removes U+%s at both ends, as Go does',
    (_, space) => {
      expect(trimSpace(`${space}${space}Obra${space}`)).toBe('Obra')
    },
  )

  it('removes U+0085, which String.prototype.trim keeps', () => {
    expect(trimSpace('\u0085')).toBe('')
  })

  it('keeps U+FEFF, which String.prototype.trim removes', () => {
    expect(trimSpace('﻿Obra﻿')).toBe('﻿Obra﻿')
  })

  it('keeps the spaces inside the text', () => {
    expect(trimSpace('  Orden  de trabajo ')).toBe('Orden  de trabajo')
  })

  it('leaves the text alone when there is nothing to trim', () => {
    expect(trimSpace('Obra')).toBe('Obra')
    expect(trimSpace('')).toBe('')
  })
})
