import { readFileSync, readdirSync } from 'node:fs'
import { spawnSync } from 'node:child_process'

const generated = () =>
  new Map(
    [
      ...readdirSync('src/api/generated').map(
        (name) => `src/api/generated/${name}`,
      ),
      'test-fixtures/openapi/generated.ts',
    ].map((path) => [path, readFileSync(path, 'utf8')]),
  )
const before = generated()
const result = spawnSync('npm', ['run', 'gen:api'], { stdio: 'inherit' })
if (result.status !== 0) process.exit(result.status ?? 1)
const changed = [...generated()].filter(
  ([path, contents]) => before.get(path) !== contents,
)
if (changed.length) {
  process.stderr.write(
    `Generated API types were stale: ${changed.map(([path]) => path).join(', ')}. Commit the regenerated files.\n`,
  )
  process.exit(1)
}
