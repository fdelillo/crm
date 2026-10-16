import { ESLint } from 'eslint'
import { describe, expect, it } from 'vitest'

// Runs the real eslint.config.js, so the architecture boundaries are protected in CI.
const eslint = new ESLint()
const IMPORTS = 'no-restricted-imports'
const SYNTAX = 'no-restricted-syntax'
const ui = 'src/components/ui/probe.tsx'
const users = 'src/features/users/probe.ts'

async function ruleHit(file: string, code: string, rule: string) {
  const results = await eslint.lintText(code, { filePath: file })
  const errors = (results[0]?.messages ?? []).filter(
    (message) => message.severity === 2,
  )
  return {
    rejected: errors.some((message) => message.ruleId === rule),
    unexpected: errors.filter((message) => message.ruleId !== rule),
  }
}
const importOf = (path: string) =>
  `import { value } from '${path}'; export const probe = value`
const lazyOf = (path: string) => `export const probe = () => import(${path})`

type Case = [name: string, file: string, code: string, rule: string]

// T-001 (M1): import boundaries and normalized paths.
const rejected: Case[] = []
for (const directory of ['api', 'features', 'app']) {
  for (const prefix of ['@/', '../', '../../', '../../../']) {
    rejected.push([
      `components reject ${prefix}${directory}`,
      ui,
      importOf(`${prefix}${directory}/client`),
      IMPORTS,
    ])
  }
}
rejected.push(
  [
    'components reject dynamic @/api/client',
    ui,
    lazyOf("'@/api/client'"),
    SYNTAX,
  ],
  [
    'components reject ./../../api/client',
    ui,
    importOf('./../../api/client'),
    IMPORTS,
  ],
  [
    'components reject @/lib/../api/client',
    ui,
    importOf('@/lib/../api/client'),
    IMPORTS,
  ],
  [
    'components reject @/./api/queryKeys',
    ui,
    importOf('@/./api/queryKeys'),
    IMPORTS,
  ],
  [
    'components reject ../.././api/queryKeys',
    ui,
    importOf('../.././api/queryKeys'),
    IMPORTS,
  ],
  ['components reject @//api/client', ui, importOf('@//api/client'), IMPORTS],
  [
    'components reject dynamic @/./api/queryKeys',
    ui,
    lazyOf("'@/./api/queryKeys'"),
    SYNTAX,
  ],
  [
    'components reject dynamic @//api/client',
    ui,
    lazyOf("'@//api/client'"),
    SYNTAX,
  ],
  [
    'components reject template dynamic @/api/client',
    ui,
    lazyOf('`@/api/client`'),
    SYNTAX,
  ],
  [
    'users reject dynamic @/features/tenant/api',
    users,
    lazyOf("'@/features/tenant/api'"),
    SYNTAX,
  ],
  [
    'users reject dynamic ../tenant/api',
    users,
    lazyOf("'../tenant/api'"),
    SYNTAX,
  ],
  [
    'users reject template dynamic @/features/tenant/api',
    users,
    lazyOf('`@/features/tenant/api`'),
    SYNTAX,
  ],
  [
    'users reject template dynamic ../tenant/api',
    users,
    lazyOf('`../tenant/api`'),
    SYNTAX,
  ],
  ['users reject ./../tenant/api', users, importOf('./../tenant/api'), IMPORTS],
  [
    'users reject ../../features/tenant/api',
    users,
    importOf('../../features/tenant/api'),
    IMPORTS,
  ],
  [
    'users reject @/features/users/../tenant/api',
    users,
    importOf('@/features/users/../tenant/api'),
    IMPORTS,
  ],
  [
    'users reject .././tenant/CompanyPage',
    users,
    importOf('.././tenant/CompanyPage'),
    IMPORTS,
  ],
  [
    'users reject @/./features/tenant/CompanyPage',
    users,
    importOf('@/./features/tenant/CompanyPage'),
    IMPORTS,
  ],
  [
    'users reject ../.././features/tenant/api',
    users,
    importOf('../.././features/tenant/api'),
    IMPORTS,
  ],
  [
    'users reject @//features/tenant/api',
    users,
    importOf('@//features/tenant/api'),
    IMPORTS,
  ],
  [
    'users reject dynamic .././tenant/api',
    users,
    lazyOf("'.././tenant/api'"),
    SYNTAX,
  ],
  [
    'users reject dynamic openapi-fetch',
    users,
    lazyOf("'openapi-fetch'"),
    SYNTAX,
  ],
  [
    'users reject template dynamic openapi-fetch',
    users,
    lazyOf('`openapi-fetch`'),
    SYNTAX,
  ],
)
const allowed: Case[] = [
  ...['@/lib/utils', '../../lib/utils', '@/components/ui/button'].map(
    (path): Case => [`components allow ${path}`, ui, importOf(path), IMPORTS],
  ),
  ['users allow lazy ./other', users, lazyOf("'./other'"), SYNTAX],
  ['users allow lazy template ./other', users, lazyOf('`./other`'), SYNTAX],
  ['users allow ../auth/session', users, importOf('../auth/session'), IMPORTS],
  [
    'users allow @/features/auth/session',
    users,
    importOf('@/features/auth/session'),
    IMPORTS,
  ],
  [
    'users allow dynamic auth/session',
    users,
    lazyOf("'@/features/auth/session'"),
    SYNTAX,
  ],
  ['users allow @/lib/utils', users, importOf('@/lib/utils'), IMPORTS],
  [
    'users allow @/components/ui/button',
    users,
    importOf('@/components/ui/button'),
    IMPORTS,
  ],
  [
    'app allow dynamic ./router',
    'src/app/probe.ts',
    lazyOf("'./router'"),
    SYNTAX,
  ],
]

// T-001 (M1): network and canvas stay in their modules.
const canvas = [
  'createImageBitmap(new Blob())',
  'window.createImageBitmap(new Blob())',
  'document.createElement("canvas")',
  'document.createElement(`canvas`)',
  'document.createElement("div").getContext("2d")',
  'new OffscreenCanvas(1, 1)',
]
const canvasModule = 'src/features/tenant/logo/canvas.ts'
for (const file of [
  'src/api/probe.ts',
  'src/features/tenant/api.ts',
  ui,
  canvasModule,
]) {
  for (const expression of canvas) {
    const entry: Case = [
      `${file}: ${expression}`,
      file,
      `export const probe = () => ${expression}`,
      SYNTAX,
    ]
    ;(file === canvasModule ? allowed : rejected).push(entry)
  }
}
for (const file of [
  'src/api/probe.ts',
  'src/features/tenant/api.ts',
  canvasModule,
  ui,
]) {
  const blocked = file === canvasModule || file === ui
  for (const call of ['fetch("/api/v1/me")', 'window.fetch("/api/v1/me")']) {
    const entry: Case = [
      `${file}: ${call}`,
      file,
      `export const probe = () => ${call}`,
      SYNTAX,
    ]
    ;(blocked ? rejected : allowed).push(entry)
  }
  const client: Case = [
    `${file}: openapi-fetch`,
    file,
    'import createClient from "openapi-fetch"; export const probe = createClient',
    IMPORTS,
  ]
  ;(blocked || file === 'src/features/tenant/api.ts' ? rejected : allowed).push(
    client,
  )
}

describe('import and network boundaries', () => {
  it.each(rejected)('rejects: %s', async (_name, file, code, rule) => {
    const hit = await ruleHit(file, code, rule)
    expect(hit.unexpected).toEqual([])
    expect(hit.rejected).toBe(true)
  })
  it.each(allowed)('allows: %s', async (_name, file, code, rule) => {
    const hit = await ruleHit(file, code, rule)
    expect(hit.unexpected).toEqual([])
    expect(hit.rejected).toBe(false)
  })
})
