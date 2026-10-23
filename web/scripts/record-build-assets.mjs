// T-F704: describe the exact SPA embedded by make build without changing its Vite config.
import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import {
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import { join, resolve } from 'node:path'
import { gzipSync } from 'node:zlib'

const [destination] = process.argv.slice(2)
if (!destination)
  throw new Error('Usage: record-build-assets.mjs OUTPUT_DIRECTORY')
const output = resolve(destination)
mkdirSync(output, { recursive: true })
const temporaryDist = join(output, 'manifest-dist')
execFileSync(
  process.execPath,
  [
    'node_modules/vite/bin/vite.js',
    'build',
    '--manifest',
    '--outDir',
    temporaryDist,
  ],
  { stdio: 'inherit' },
)
const hash = (contents) => createHash('sha256').update(contents).digest('hex')
const files = (directory, prefix = '') =>
  readdirSync(join(directory, prefix), { withFileTypes: true }).flatMap(
    (entry) => {
      const name = join(prefix, entry.name)
      if (entry.isDirectory()) return files(directory, name)
      return name === '.gitkeep' || name.startsWith('.vite/') ? [] : [name]
    },
  )
const embeddedFiles = files('dist').sort()
const manifestFiles = files(temporaryDist).sort()
if (JSON.stringify(embeddedFiles) !== JSON.stringify(manifestFiles)) {
  throw new Error(
    'The manifest build does not have the embedded build file names',
  )
}
const assets = embeddedFiles.map((name) => {
  const contents = readFileSync(join('dist', name))
  const sha256 = hash(contents)
  if (sha256 !== hash(readFileSync(join(temporaryDist, name)))) {
    throw new Error(
      `The manifest build differs from the embedded build: ${name}`,
    )
  }
  return {
    name,
    sha256,
    bytes: contents.length,
    gzipBytes: gzipSync(contents).length,
  }
})
writeFileSync(
  join(output, 'manifest.json'),
  readFileSync(join(temporaryDist, '.vite/manifest.json')),
)
writeFileSync(
  join(output, 'build-identity.json'),
  JSON.stringify(
    {
      sourceSha: execFileSync('git', ['rev-parse', 'HEAD'], {
        encoding: 'utf8',
      }).trim(),
      binarySha256: hash(readFileSync('../bin/crm')),
      node: process.version,
      vite: JSON.parse(readFileSync('node_modules/vite/package.json', 'utf8'))
        .version,
      manifestMatchesEmbeddedAssets: true,
      assets,
    },
    null,
    2,
  ) + '\n',
)
rmSync(temporaryDist, { recursive: true })
