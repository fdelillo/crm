import { ESLint } from 'eslint'
import { readFileSync } from 'node:fs'

import { describe, expect, it } from 'vitest'

import { stageColorNames } from '@/features/configuration/stages/labels'

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

// T-002-F201 (002/ui.md §19.2): every stage color of the contract has its token, exposed to Tailwind,
// and each one is at least 3:1 against --card (1.4.11, also for decorative swatches).
const css = readFileSync('src/styles/globals.css', 'utf8')

function cssValue(name: string): string {
  const match = new RegExp(`^\\s*${name}:\\s*(#[0-9a-fA-F]{6});`, 'm').exec(css)
  if (!match?.[1]) throw new Error(`No ${name} in globals.css`)
  return match[1]
}

function luminance(hex: string): number {
  const [r = 0, g = 0, b = 0] = [1, 3, 5].map((i) => {
    const channel = parseInt(hex.slice(i, i + 2), 16) / 255
    return channel <= 0.03928
      ? channel / 12.92
      : ((channel + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(a: string, b: string): number {
  const [hi = 0, lo = 0] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

describe('stage color tokens', () => {
  const colors = Object.keys(stageColorNames)
  it.each(colors)('--stage-%s is exposed as bg-stage-%s', (color) => {
    expect(css).toContain(`--color-stage-${color}: var(--stage-${color});`)
  })
  it.each(colors)('--stage-%s has at least 3:1 against --card', (color) => {
    expect(
      contrast(cssValue(`--stage-${color}`), cssValue('--card')),
    ).toBeGreaterThanOrEqual(3)
  })
})
