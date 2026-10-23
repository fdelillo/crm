// T-F702: restart only the CI-owned CRM process between registration-heavy test groups.
import { spawn } from 'node:child_process'
import {
  existsSync,
  openSync,
  readFileSync,
  readlinkSync,
  rmSync,
  writeFileSync,
  closeSync,
} from 'node:fs'
import { join } from 'node:path'
import { setTimeout } from 'node:timers/promises'
import { fileURLToPath } from 'node:url'

const [action] = process.argv.slice(2)
if (!['start', 'restart', 'stop'].includes(action)) {
  throw new Error(
    'Usage: manage-e2e-server.mjs start|restart|stop (GitHub Actions Linux runner only)',
  )
}
if (
  process.platform !== 'linux' ||
  process.env.GITHUB_ACTIONS !== 'true' ||
  !process.env.RUNNER_TEMP
) {
  throw new Error(
    'This helper manages only its GitHub Actions process; local E2E uses an external server',
  )
}
const root = fileURLToPath(new URL('../../', import.meta.url))
const executable = join(root, 'bin/crm')
const stateFile = join(process.env.RUNNER_TEMP, 'crm-e2e-process.json')
const logFile = join(process.env.RUNNER_TEMP, 'crm-e2e.log')
const processState = (pid) => {
  try {
    const stat = readFileSync(`/proc/${pid}/stat`, 'utf8')
    // The command in parentheses can contain spaces; fields after it begin with state (field 3).
    const fields = stat
      .slice(stat.lastIndexOf(') ') + 2)
      .trim()
      .split(/\s+/)
    if (fields[0] === 'Z') return null
    return {
      startTicks: fields[19],
      executable: readlinkSync(`/proc/${pid}/exe`),
    }
  } catch (error) {
    if (error.code === 'ENOENT' || error.code === 'ESRCH') return null
    throw error
  }
}
const matches = (current, expected) =>
  current?.startTicks === expected.startTicks &&
  current?.executable === executable
const stop = async () => {
  if (!existsSync(stateFile)) return
  const owned = JSON.parse(readFileSync(stateFile, 'utf8'))
  if (
    !Number.isSafeInteger(owned.pid) ||
    owned.pid <= 1 ||
    !/^\d+$/.test(owned.startTicks)
  ) {
    throw new Error(
      'Invalid CI-owned process state; refusing to signal any process',
    )
  }
  const current = processState(owned.pid)
  if (current && !matches(current, owned)) {
    throw new Error(
      'The recorded PID has a different identity; refusing to signal it',
    )
  }
  if (current) {
    try {
      process.kill(owned.pid, 'SIGTERM')
    } catch (error) {
      if (error.code !== 'ESRCH') throw error
    }
    for (let attempt = 0; attempt < 120; attempt++) {
      const remaining = processState(owned.pid)
      if (!remaining || !matches(remaining, owned)) break
      if (attempt === 119)
        throw new Error('The CI-owned CRM did not stop within 60 seconds')
      await setTimeout(500)
    }
  }
  rmSync(stateFile)
  process.stdout.write(`Stopped CI-owned CRM PID ${owned.pid}\n`)
}
const start = async () => {
  if (existsSync(stateFile))
    throw new Error('A CI-owned process is already recorded; stop it first')
  const log = openSync(logFile, 'a')
  const child = spawn(executable, ['serve'], {
    cwd: root,
    detached: true,
    stdio: ['ignore', log, log],
    env: process.env,
  })
  closeSync(log)
  await new Promise((resolve, reject) => {
    child.once('spawn', resolve)
    child.once('error', reject)
  })
  child.unref()
  const current = processState(child.pid)
  if (!current || current.executable !== executable)
    throw new Error('The CRM child exited before its identity was recorded')
  const owned = { pid: child.pid, startTicks: current.startTicks }
  writeFileSync(stateFile, JSON.stringify(owned) + '\n', {
    mode: 0o600,
    flag: 'wx',
  })
  for (let attempt = 0; attempt < 60; attempt++) {
    if (!matches(processState(owned.pid), owned))
      throw new Error(`The CI-owned CRM exited; see ${logFile}`)
    let healthy = false
    try {
      const response = await fetch('https://localhost:8443/healthz', {
        signal: AbortSignal.timeout(1000),
      })
      healthy = response.ok
      await response.body?.cancel()
    } catch {
      // Readiness is retried only while the exact child process is still alive.
    }
    if (healthy && matches(processState(owned.pid), owned)) {
      process.stdout.write(`Started healthy CI-owned CRM PID ${owned.pid}\n`)
      return
    }
    await setTimeout(1000)
  }
  throw new Error(`The CI-owned CRM did not become healthy; see ${logFile}`)
}
if (action === 'restart' && !existsSync(stateFile)) {
  throw new Error(
    'No CI-owned CRM PID was recorded; refusing to restart an external server',
  )
}
if (action === 'stop' || action === 'restart') await stop()
if (action === 'start' || action === 'restart') await start()
