import js from '@eslint/js'
import globals from 'globals'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'
import jsxA11y from 'eslint-plugin-jsx-a11y'

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
      "CallExpression[callee.property.name='createElement'][arguments.0.value='canvas']",
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
const forbiddenNetworkImports = {
  name: 'openapi-fetch',
  message: 'HTTP clients belong in api/.',
}
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
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    rules: { 'no-console': 'error' },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: { 'react-hooks': reactHooks, 'jsx-a11y': jsxA11y },
    rules: {
      ...reactHooks.configs.recommended.rules,
      ...jsxA11y.configs.recommended.rules,
    },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/api/**', '**/*.test.*', 'src/test/**'],
    rules: {
      'no-restricted-imports': ['error', { paths: [forbiddenNetworkImports] }],
    },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: [
      'src/api/**',
      'src/features/tenant/api.ts',
      '**/*.test.*',
      'src/test/**',
    ],
    rules: {
      'no-restricted-syntax': [
        'error',
        ...networkSelectors,
        ...canvasSelectors,
      ],
    },
  },
  {
    files: ['src/api/**/*.{ts,tsx}', 'src/features/tenant/api.ts'],
    ignores: ['**/*.test.*'],
    rules: { 'no-restricted-syntax': ['error', ...canvasSelectors] },
  },
  {
    files: ['src/features/tenant/logo/canvas.ts'],
    rules: { 'no-restricted-syntax': ['error', ...networkSelectors] },
  },
  {
    files: ['src/components/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [forbiddenNetworkImports],
          patterns: [
            {
              regex: '^(?:@/|(?:\\.\\./)+)(?:api|features|app)(?:/|$)',
              message: 'Shared components receive data through props.',
            },
          ],
        },
      ],
    },
  },
  ...['auth', 'dashboard', 'users', 'tenant'].map((feature) => ({
    files: [`src/features/${feature}/**/*.{ts,tsx}`],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [forbiddenNetworkImports],
          patterns: [
            {
              regex: `^@/features/(?!(?:${feature})(?:/|$)|auth/session(?:$|\\.))`,
              message: 'Features may only share the auth/session module.',
            },
            {
              regex: `^(?:\\.\\./)+(?!${feature}/|auth/session(?:$|\\.))(?:auth|dashboard|users|tenant)/`,
              message: 'Features may only share the auth/session module.',
            },
          ],
        },
      ],
    },
  })),
)
