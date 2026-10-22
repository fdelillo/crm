// T-F704: fail closed before uploading anonymous performance evidence; never redact metrics.
import { lstatSync, readFileSync, readdirSync } from 'node:fs'
import { extname, join, resolve } from 'node:path'

const [directory] = process.argv.slice(2)
if (!directory)
  throw new Error('Usage: validate-performance-evidence.mjs DIRECTORY')
const sensitiveHeader =
  /^(?:cookie|set-cookie|authorization|proxy-authorization)$/i
const sensitiveParameter =
  /^(?:token|code|password|secret|access_token|refresh_token|invite|session)$/i
const checkUrls = (text, file) => {
  for (const match of text.matchAll(/https?:\/\/[^\s"<>\\]+/g)) {
    let url
    try {
      url = new URL(match[0])
    } catch {
      continue
    }
    if (url.username || url.password)
      throw new Error(`URL credentials in ${file}`)
    for (const [key, value] of url.searchParams) {
      if (sensitiveParameter.test(key) && value) {
        throw new Error(`Sensitive URL parameter ${key} in ${file}`)
      }
    }
  }
}
const checkObject = (value, file) => {
  if (typeof value === 'string') {
    checkUrls(value, file)
    if (
      /(?:^|[\r\n])(?:Cookie|Set-Cookie|Authorization|Proxy-Authorization)\s*:\s*\S/i.test(
        value,
      )
    ) {
      throw new Error(`Sensitive raw HTTP header in ${file}`)
    }
    return
  }
  if (!value || typeof value !== 'object') return
  // CDP can encode headers as either a dictionary or {name, value} pairs.
  if (sensitiveHeader.test(value.name ?? '') && value.value) {
    throw new Error(`Sensitive HTTP header in ${file}`)
  }
  for (const [key, entry] of Object.entries(value)) {
    if (sensitiveHeader.test(key) && entry && JSON.stringify(entry) !== '[]') {
      throw new Error(`Sensitive HTTP header ${key} in ${file}`)
    }
    checkObject(entry, file)
  }
}
let count = 0
const inspect = (path) => {
  const stat = lstatSync(path)
  if (stat.isSymbolicLink()) throw new Error(`Symlink in evidence: ${path}`)
  if (stat.isDirectory()) {
    for (const name of readdirSync(path)) inspect(join(path, name))
    return
  }
  if (!['.json', '.html', '.txt', '.log', '.png'].includes(extname(path))) {
    throw new Error(`Unexpected evidence file: ${path}`)
  }
  count++
  if (extname(path) === '.png') return
  const contents = readFileSync(path, 'utf8')
  if (/-----BEGIN (?:[A-Z ]*PRIVATE KEY|CERTIFICATE)-----/.test(contents)) {
    throw new Error(`Certificate or private key in ${path}`)
  }
  if (extname(path) === '.json') checkObject(JSON.parse(contents), path)
  else checkUrls(contents, path)
  // The CLI help contains a fictitious Cookie example; it is documentation, not a request.
  if (!path.endsWith('lighthouse-help.txt') && extname(path) === '.log') {
    if (
      /\b(?:Cookie|Set-Cookie|Authorization|Proxy-Authorization)\s*:\s*\S/i.test(
        contents,
      )
    ) {
      throw new Error(`Sensitive HTTP header in ${path}`)
    }
  }
}
inspect(resolve(directory))
if (count === 0) throw new Error('No performance evidence to inspect')
process.stdout.write(
  `Safe to upload: ${count} evidence files; no credentials or session/link tokens found.\n`,
)
