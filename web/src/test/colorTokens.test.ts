import { ESLint } from 'eslint'
import { describe, expect, it } from 'vitest'

// T-F707 (INV-F23, ui.md §19.1): components paint with the design tokens, never with
// Tailwind palette classes. Runs the real eslint.config.js, like importBoundaries.test.ts.
const eslint = new ESLint()
const SYNTAX = 'no-restricted-syntax'

async function hit(file: string, code: string) {
  const results = await eslint.lintText(code, { filePath: file })
  if (results.length !== 1) throw new Error(`Expected one result for ${file}`)
  // An ignored file or one without the rule would otherwise look "allowed".
  const config = await eslint.calculateConfigForFile(file)
  const level = config?.rules?.[SYNTAX]?.[0]
  if (config === undefined || (level !== 2 && level !== 'error')) {
    throw new Error(`${SYNTAX} is not an error for ${file}`)
  }
  const messages = results[0]?.messages ?? []
  return {
    rejected: messages.some(
      (message) => message.ruleId === SYNTAX && message.severity === 2,
    ),
    unexpected: messages.filter((message) => message.ruleId !== SYNTAX),
  }
}

type Case = [name: string, file: string, code: string]
const rejected: Case[] = [
  [
    'a hover state with a palette color',
    'src/components/ui/probe.tsx',
    'export const probe = <button className="hover:bg-blue-700" />',
  ],
  [
    'a background in a plain string',
    'src/features/users/probe.ts',
    "export const probe = 'bg-red-600 text-white'",
  ],
  [
    'a palette color inside a template literal',
    'src/app/probe.tsx',
    'export const probe = (tone: string) => `text-slate-500 ${tone}`',
  ],
  [
    'a palette color with opacity',
    'src/components/ui/probe.tsx',
    "export const probe = 'ring-amber-400/50'",
  ],
]
const allowed: Case[] = [
  [
    'the hover token',
    'src/components/ui/probe.tsx',
    'export const probe = <button className="hover:bg-primary-hover" />',
  ],
  [
    'the dialog backdrop (no scale)',
    'src/components/ui/probe.tsx',
    "export const probe = 'bg-black/50'",
  ],
  [
    'a muted foreground token',
    'src/components/ui/probe.tsx',
    "export const probe = 'text-muted-foreground'",
  ],
]

describe('color tokens', () => {
  it.each(rejected)(
    'rejects: %s',
    async (_name, file, code) => {
      const result = await hit(file, code)
      expect(result.unexpected).toEqual([])
      expect(result.rejected).toBe(true)
    },
    30_000,
  )
  it.each(allowed)(
    'allows: %s',
    async (_name, file, code) => {
      const result = await hit(file, code)
      expect(result.unexpected).toEqual([])
      expect(result.rejected).toBe(false)
    },
    30_000,
  )
})
