import js from '@eslint/js'
import globals from 'globals'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'
import jsxA11y from 'eslint-plugin-jsx-a11y'
import { readdirSync } from 'node:fs'

const networkSelectors = [
  {
    selector: "CallExpression[callee.name='fetch']",
    message: 'HTTP requests belong in api/.',
  },
  {
    selector: "CallExpression[callee.property.name='fetch']",
    message: 'HTTP requests belong in api/.',
  },
]
const canvasSelectors = [
  {
    selector: "CallExpression[callee.name='createImageBitmap']",
    message: 'Image decoding belongs in tenant/logo/canvas.ts.',
  },
  {
    selector: "CallExpression[callee.property.name='createImageBitmap']",
    message: 'Image decoding belongs in tenant/logo/canvas.ts.',
  },
  {
    selector: "CallExpression[callee.property.name='getContext']",
    message: 'Canvas belongs in tenant/logo/canvas.ts.',
  },
  {
    selector:
      "CallExpression[callee.property.name='createElement'][arguments.0.value=/^canvas$/i]",
    message: 'Canvas belongs in tenant/logo/canvas.ts.',
  },
  {
    selector:
      "CallExpression[callee.property.name='createElement'][arguments.0.expressions.length=0][arguments.0.quasis.0.value.cooked=/^canvas$/i]",
    message: 'Canvas belongs in tenant/logo/canvas.ts.',
  },
  {
    selector: "JSXOpeningElement[name.name='canvas']",
    message: 'Canvas belongs in tenant/logo/canvas.ts.',
  },
  {
    selector: "NewExpression[callee.name='OffscreenCanvas']",
    message: 'Canvas belongs in tenant/logo/canvas.ts.',
  },
]
const restrictedBlocker = {
  name: 'react-router',
  importNames: ['useBlocker'],
  message:
    'Use lib/unsavedChanges.ts so session expiration bypasses unsaved-change prompts (INV-F18).',
}
const forbiddenNetworkImports = {
  name: 'openapi-fetch',
  message: 'HTTP clients belong in api/.',
}
// Slashes are written as \x2f so the same source works in ESLint selectors,
// whose regex literals cannot contain a "/".
const rx = (source) => source.replaceAll('/', '\\x2f')
// INV-F23: components paint with the tokens of globals.css, never with the Tailwind
// palette (bg-blue-700, text-slate-500). Opacity over a token (bg-primary/90) is not linted.
const paletteClass = String.raw`(?:^|[\s:!-])(?:bg|text|border|ring|outline|fill|stroke|from|via|to|divide|decoration|accent|caret|shadow|placeholder)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d{2,3}(?:$|[\s/])`
const colorMessage =
  'Use the color tokens of globals.css (bg-primary-hover, text-muted-foreground), not the Tailwind palette (INV-F23).'
const colorSelectors = [
  { selector: `Literal[value=/${rx(paletteClass)}/]`, message: colorMessage },
  {
    selector: `TemplateElement[value.cooked=/${rx(paletteClass)}/]`,
    message: colorMessage,
  },
]
const features = readdirSync(new URL('./src/features', import.meta.url), {
  withFileTypes: true,
})
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)
const sharedFeatureModule = 'auth/session(?:$|\\.)'
// A ".." right after a named segment can hide the real destination.
const hiddenParentSegment = {
  regex: rx('(?:^|/)[^./][^/]*/\\.\\.(?:/|$)'),
  message:
    'Do not climb with ".." after a named segment; import the target directly.',
}
// "./" in the middle or an empty segment ("//") can also hide the destination.
const nonNormalizedPath = {
  regex: rx('.(?:/\\.(?:/|$)|//)'),
  message: 'Import the normalized path (no "./" in the middle, no "//").',
}
// Climbing out of src/ and coming back in (../../../src/..., ../../../../web/src/...).
const reenteredRoot = {
  regex: rx('^(?:\\./)*(?:\\.\\./)+(?:src|web)(?:/|$)'),
  message: 'Import with the "@/" alias instead of re-entering src/ or web/.',
}
// Known limit: import() with expressions (`@/api/${x}`, concatenations) cannot
// be checked statically; only literals and template literals without expressions are.
const componentImportRegex = rx(
  '^(?:@/|(?:\\./)*(?:\\.\\./)+)(?:api|features|app)(?:/|$)',
)
const componentPatterns = [
  {
    regex: componentImportRegex,
    message: 'Shared components receive data through props.',
  },
  hiddenParentSegment,
  nonNormalizedPath,
  reenteredRoot,
]
const featurePatterns = (feature) => [
  {
    regex: rx(`^@/features/(?!(?:${feature})(?:/|$)|${sharedFeatureModule})`),
    message: 'Features may only share the auth/session module.',
  },
  {
    regex: rx(
      `^(?:\\./)*(?:\\.\\./)+(?!${feature}/|${sharedFeatureModule})(?:${features.join('|')})/`,
    ),
    message: 'Features may only share the auth/session module.',
  },
  {
    regex: rx(
      `^(?:\\./)*(?:\\.\\./)+features/(?!(?:${feature})(?:/|$)|${sharedFeatureModule})`,
    ),
    message: 'Features may only share the auth/session module.',
  },
  hiddenParentSegment,
  nonNormalizedPath,
  reenteredRoot,
]
// Covers 'x' and also a template literal without expressions: `x`.
const dynamicImport = (regex, message) => [
  { selector: `ImportExpression[source.value=/${regex}/]`, message },
  {
    selector: `ImportExpression[source.expressions.length=0][source.quasis.0.value.cooked=/${regex}/]`,
    message,
  },
]
const dynamicImports = (patterns) =>
  patterns.flatMap(({ regex, message }) => dynamicImport(regex, message))
const dynamicClientImport = dynamicImport(
  rx('^openapi-fetch$'),
  'HTTP clients belong in api/.',
)
const syntaxRule = (...groups) => ['error', ...groups.flat()]
// INV-F22: its subscription decides when a finished session exit is cleared, so only
// RequireSession (routeGuards.tsx, exempted below) may use it.
const sessionLifecycleOnlyInGuard = {
  regex: rx('(?:^|/)sessionExit$'),
  importNames: ['useSessionLifecycle'],
  message: 'Only RequireSession reads the session lifecycle (INV-F22).',
}
const noRestrictedImports = (patterns) => [
  'error',
  {
    paths: [forbiddenNetworkImports, restrictedBlocker],
    patterns: [...patterns, sessionLifecycleOnlyInGuard],
  },
]
const testFiles = ['**/*.test.*', 'src/test/**']
export default tseslint.config(
  {
    ignores: [
      'dist/**',
      'node_modules/**',
      'src/api/generated/**',
      'test-fixtures/openapi/generated.ts',
      '.api-bundle/**',
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{js,mjs,ts,tsx}'],
    languageOptions: { globals: globals.browser },
    rules: { 'no-console': 'error' },
  },
  {
    files: ['*.config.*', 'e2e/**/*.{js,mjs,ts}', 'scripts/**/*.{js,mjs,ts}'],
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: { 'react-hooks': reactHooks, 'jsx-a11y': jsxA11y },
    rules: {
      ...reactHooks.configs.recommended.rules,
      ...jsxA11y.configs.recommended.rules,
      'no-restricted-imports': [
        'error',
        {
          paths: [restrictedBlocker],
          patterns: [sessionLifecycleOnlyInGuard],
        },
      ],
    },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/api/**', ...testFiles],
    rules: {
      'no-restricted-imports': noRestrictedImports([]),
      'no-restricted-syntax': syntaxRule(
        networkSelectors,
        canvasSelectors,
        dynamicClientImport,
        colorSelectors,
      ),
    },
  },
  {
    files: ['src/api/**/*.{ts,tsx}'],
    ignores: ['**/*.test.*'],
    rules: {
      'no-restricted-syntax': syntaxRule(canvasSelectors, colorSelectors),
    },
  },
  {
    files: ['src/components/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': noRestrictedImports(componentPatterns),
      'no-restricted-syntax': syntaxRule(
        dynamicClientImport,
        dynamicImports(componentPatterns),
      ),
    },
  },
  {
    files: ['src/components/**/*.{ts,tsx}'],
    ignores: testFiles,
    rules: {
      'no-restricted-syntax': syntaxRule(
        networkSelectors,
        canvasSelectors,
        dynamicClientImport,
        dynamicImports(componentPatterns),
        colorSelectors,
      ),
    },
  },
  ...features.flatMap((feature) => {
    const patterns = featurePatterns(feature)
    const dir = `src/features/${feature}`
    return [
      {
        files: [`${dir}/**/*.{ts,tsx}`],
        rules: {
          'no-restricted-imports': noRestrictedImports(patterns),
          'no-restricted-syntax': syntaxRule(
            dynamicClientImport,
            dynamicImports(patterns),
          ),
        },
      },
      {
        files: [`${dir}/**/*.{ts,tsx}`],
        ignores: testFiles,
        rules: {
          'no-restricted-syntax': syntaxRule(
            networkSelectors,
            canvasSelectors,
            dynamicClientImport,
            dynamicImports(patterns),
            colorSelectors,
          ),
        },
      },
    ]
  }),
  {
    files: ['src/lib/unsavedChanges.ts'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [forbiddenNetworkImports],
          patterns: [sessionLifecycleOnlyInGuard],
        },
      ],
    },
  },
  {
    files: ['src/app/guards/routeGuards.tsx'],
    rules: {
      'no-restricted-imports': [
        'error',
        { paths: [forbiddenNetworkImports, restrictedBlocker] },
      ],
    },
  },
  // The tenant HTTP module may call fetch; the logo module is the only canvas user.
  {
    files: ['src/features/tenant/api.ts'],
    ignores: ['**/*.test.*'],
    rules: {
      'no-restricted-syntax': syntaxRule(
        canvasSelectors,
        dynamicClientImport,
        dynamicImports(featurePatterns('tenant')),
        colorSelectors,
      ),
    },
  },
  {
    files: ['src/features/tenant/logo/canvas.ts'],
    rules: {
      'no-restricted-syntax': syntaxRule(
        networkSelectors,
        dynamicClientImport,
        dynamicImports(featurePatterns('tenant')),
        colorSelectors,
      ),
    },
  },
)
