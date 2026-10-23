// T-F704: repeated, interleaved anonymous measurements; NFR-F03 is reported, never weakened.
import { createHash } from 'node:crypto'
import { execFileSync, spawnSync } from 'node:child_process'
import {
  closeSync,
  existsSync,
  mkdirSync,
  openSync,
  readFileSync,
  writeFileSync,
} from 'node:fs'
import { arch, cpus, freemem, platform, release, totalmem } from 'node:os'
import { join, resolve } from 'node:path'
import { chromium } from 'playwright'

const [directory] = process.argv.slice(2)
if (!directory)
  throw new Error('Usage: collect-performance-evidence.mjs EVIDENCE_DIRECTORY')
const output = resolve(directory)
mkdirSync(output, { recursive: true })
const save = (name, value) =>
  writeFileSync(join(output, name), JSON.stringify(value, null, 2) + '\n')
const sha256 = (contents) => createHash('sha256').update(contents).digest('hex')
const sourceSha = execFileSync('git', ['rev-parse', 'HEAD'], {
  encoding: 'utf8',
}).trim()
const build = JSON.parse(
  readFileSync(join(output, 'build/build-identity.json'), 'utf8'),
)
if (
  sourceSha !== build.sourceSha ||
  sha256(readFileSync('../bin/crm')) !== build.binarySha256
) {
  throw new Error(
    'The checked out revision or downloaded binary differs from the manifest build',
  )
}
// Compare the actual HTTPS server with the recorded assets, independently of the download hash.
for (const asset of build.assets) {
  const response = await fetch(`https://localhost:8443/${asset.name}`, {
    headers: { 'Accept-Encoding': 'identity' },
  })
  if (
    !response.ok ||
    sha256(Buffer.from(await response.arrayBuffer())) !== asset.sha256
  ) {
    throw new Error(
      `The served asset differs from the embedded build: ${asset.name}`,
    )
  }
}
const chromePath = chromium.executablePath()
const cli = resolve('node_modules/lighthouse/cli/index.js')
const help = execFileSync(process.execPath, [cli, '--help'], {
  encoding: 'utf8',
})
if (!help.includes('--save-assets'))
  throw new Error('This Lighthouse CLI does not support --save-assets')
writeFileSync(join(output, 'lighthouse-help.txt'), help)
const lighthouseVersion = JSON.parse(
  readFileSync('node_modules/lighthouse/package.json', 'utf8'),
).version
if (lighthouseVersion !== '13.5.0')
  throw new Error(`Expected Lighthouse 13.5.0, found ${lighthouseVersion}`)
const environment = {
  sourceSha,
  pullRequestHeadSha: process.env.PR_HEAD_SHA ?? null,
  binarySha256: build.binarySha256,
  lighthouseVersion,
  node: process.version,
  chrome: execFileSync(chromePath, ['--version'], { encoding: 'utf8' }).trim(),
  runner: {
    os: process.env.RUNNER_OS ?? platform(),
    architecture: process.env.RUNNER_ARCH ?? arch(),
    imageOS: process.env.ImageOS ?? null,
    imageVersion: process.env.ImageVersion ?? null,
    kernel: release(),
    cpu: cpus().map(({ model, speed }) => ({ model, speed })),
    memoryBytes: totalmem(),
    freeMemoryBytesAtStart: freemem(),
    githubRunId: process.env.GITHUB_RUN_ID ?? null,
    githubRunAttempt: process.env.GITHUB_RUN_ATTEMPT ?? null,
    job: process.env.GITHUB_JOB ?? null,
  },
  auditAliases: {
    'lcp-phases-insight':
      'lcp-breakdown-insight (the audit ID shipped by Lighthouse 13.5.0)',
  },
  notes: [
    'Every Lighthouse CLI invocation launches its own temporary anonymous Chrome profile.',
    'No existing Playwright session, cookies, authentication headers, or storage are reused.',
    'Trace timestamps and LCP breakdown are observed; simulated LCP/FCP are separate model outputs.',
    'The two devtools-throttled runs are diagnostic only and excluded from NFR-F03 statistics.',
    'One discarded simulate warm-up per route runs first and is excluded from NFR-F03 statistics.',
  ],
}
save('environment.json', environment)
// T-F704: the first run of the job meets a cold runner (benchmarkIndex ~1760-1790 against ~2400
// later) and Lantern scales that CPU by 4, so it is kept as evidence but never counted.
const plan = [
  { method: 'simulate', pair: 0, route: 'login', warmup: true },
  { method: 'simulate', pair: 0, route: 'root', warmup: true },
]
for (let pair = 1; pair <= 5; pair++) {
  for (const route of pair % 2 ? ['login', 'root'] : ['root', 'login']) {
    plan.push({ method: 'simulate', pair, route })
  }
}
for (const route of ['login', 'root'])
  plan.push({ method: 'devtools', pair: 1, route })
const runs = []
save(
  'run-plan.json',
  plan.map((run, index) => ({ order: index + 1, ...run })),
)
const requiredAudits = [
  'largest-contentful-paint',
  'first-contentful-paint',
  'metrics',
  'lcp-breakdown-insight',
  'network-dependency-tree-insight',
  'render-blocking-insight',
  'network-requests',
  'bootup-time',
  'mainthread-work-breakdown',
  'screenshot-thumbnails',
  'final-screenshot',
]
try {
  for (const [index, run] of plan.entries()) {
    const prefix = `${run.warmup ? 'warmup' : run.method}-${String(run.pair).padStart(2, '0')}-${run.route}`
    const url = `https://localhost:8443/${run.route === 'login' ? 'login' : ''}`
    const args = [
      cli,
      url,
      `--chrome-path=${chromePath}`,
      '--chrome-flags=--headless=new --no-sandbox',
      '--preset=perf',
      '--form-factor=mobile',
      `--throttling-method=${run.method}`,
      '--only-categories=performance',
      '--output=json',
      '--output=html',
      '--save-assets',
      `--output-path=${join(output, prefix)}`,
    ]
    const record = {
      order: index + 1,
      ...run,
      prefix,
      url,
      sourceSha,
      lighthouseVersion,
      startedAt: new Date().toISOString(),
      command: [process.execPath, ...args],
    }
    runs.push(record)
    save('runs.json', runs)
    process.stdout.write(
      `Lighthouse ${record.order}/${plan.length}: ${run.warmup ? 'warm-up' : run.method} ${url}\n`,
    )
    const log = openSync(join(output, `${prefix}.log`), 'w')
    let result
    try {
      result = spawnSync(process.execPath, args, {
        stdio: ['ignore', log, log],
        timeout: 180000,
      })
    } finally {
      closeSync(log)
    }
    record.finishedAt = new Date().toISOString()
    record.exitCode = result.status
    if (result.error || result.status !== 0) {
      record.error =
        result.error?.message ??
        `CLI exited ${result.status}; see ${prefix}.log`
      throw new Error(record.error)
    }
    for (const suffix of [
      '.report.json',
      '.report.html',
      '-0.trace.json',
      '-0.devtoolslog.json',
    ]) {
      if (!existsSync(join(output, prefix + suffix)))
        throw new Error(`Missing evidence: ${prefix}${suffix}`)
    }
    const report = JSON.parse(
      readFileSync(join(output, `${prefix}.report.json`), 'utf8'),
    )
    if (report.runtimeError)
      throw new Error(`Lighthouse runtime error: ${report.runtimeError.code}`)
    if (
      report.lighthouseVersion !== lighthouseVersion ||
      report.configSettings.throttlingMethod !== run.method
    ) {
      throw new Error(
        'Lighthouse used an unexpected version or throttling method',
      )
    }
    for (const audit of requiredAudits) {
      if (!report.audits[audit] || report.audits[audit].errorMessage) {
        throw new Error(`Missing or failed audit ${audit} in ${prefix}`)
      }
    }
    if (!Number.isFinite(report.environment.benchmarkIndex))
      throw new Error('Missing benchmarkIndex')
    const metrics = report.audits.metrics.details.items[0]
    record.environment = report.environment
    record.configSettings = report.configSettings
    record.benchmarkIndex = report.environment.benchmarkIndex
    record.performanceScore = report.categories.performance.score
    record.lcpMs = report.audits['largest-contentful-paint'].numericValue
    record.fcpMs = report.audits['first-contentful-paint'].numericValue
    record.observedLcpMs = metrics.observedLargestContentfulPaint
    record.observedFcpMs = metrics.observedFirstContentfulPaint
    record.cls = report.audits['cumulative-layout-shift'].numericValue
    record.runWarnings = report.runWarnings
    save('runs.json', runs)
  }
  const statistics = {}
  for (const route of ['login', 'root']) {
    const routeRuns = runs.filter(
      (run) => run.route === route && run.method === 'simulate' && !run.warmup,
    )
    const warmup = runs.find((run) => run.route === route && run.warmup)
    const sorted = routeRuns.map((run) => run.lcpMs).sort((a, b) => a - b)
    statistics[route] = {
      samples: sorted,
      medianLcpMs: sorted[2],
      p75LcpMs: sorted[3],
      percentileMethod:
        'nearest rank: ceil(0.75 * 5), fourth of five sorted simulated samples',
      benchmarkIndex: routeRuns.map((run) => run.benchmarkIndex),
      observedLcpMs: routeRuns.map((run) => run.observedLcpMs),
      discardedWarmup: {
        lcpMs: warmup.lcpMs,
        benchmarkIndex: warmup.benchmarkIndex,
      },
    }
  }
  save('statistics.json', statistics)
} finally {
  save('runs.json', runs)
}
