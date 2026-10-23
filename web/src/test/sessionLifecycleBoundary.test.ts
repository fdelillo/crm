import { ESLint } from 'eslint'
import { describe, expect, it } from 'vitest'

// T-F707 (INV-F22): only RequireSession reads the session lifecycle, because its subscription
// decides when a finished exit is cleared. Runs the real eslint.config.js.
const eslint = new ESLint()
const RULE = 'no-restricted-imports'

async function rejected(file: string, code: string) {
  const [result] = await eslint.lintText(code, { filePath: file })
  const config = await eslint.calculateConfigForFile(file)
  const level = config?.rules?.[RULE]?.[0]
  if (level !== 2 && level !== 'error')
    throw new Error(`${RULE} is not an error for ${file}`)
  const messages = result?.messages ?? []
  return {
    hit: messages.some((m) => m.ruleId === RULE && m.severity === 2),
    unexpected: messages.filter((m) => m.ruleId !== RULE),
  }
}
const importOf = (path: string, name = 'useSessionLifecycle') =>
  `import { ${name} } from '${path}'; export const probe = ${name}`

const cases: [string, string, string, boolean][] = [
  [
    'a feature',
    'src/features/users/probe.ts',
    importOf('@/lib/sessionExit'),
    true,
  ],
  [
    'a component',
    'src/components/probe.tsx',
    importOf('@/lib/sessionExit'),
    true,
  ],
  ['the shell', 'src/app/probe.tsx', importOf('@/lib/sessionExit'), true],
  ['a relative import', 'src/lib/probe.ts', importOf('./sessionExit'), true],
  [
    'a renamed import',
    'src/app/probe.tsx',
    "import { useSessionLifecycle as read } from '@/lib/sessionExit'; export const probe = read",
    true,
  ],
  [
    'RequireSession',
    'src/app/guards/routeGuards.tsx',
    importOf('@/lib/sessionExit'),
    false,
  ],
  [
    'other exports of the module',
    'src/features/auth/probe.ts',
    importOf('@/lib/sessionExit', 'exitSession'),
    false,
  ],
]
describe('useSessionLifecycle boundary', () => {
  it.each(cases)(
    '%s',
    async (_name, file, code, expected) => {
      const result = await rejected(file, code)
      expect(result.unexpected).toEqual([])
      expect(result.hit).toBe(expected)
    },
    30_000,
  )
})
