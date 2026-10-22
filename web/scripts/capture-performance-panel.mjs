// T-F704: open saved traces in Chrome's own Performance panel, using the existing Playwright.
import { readFileSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { chromium } from 'playwright'

const [directory] = process.argv.slice(2)
if (!directory)
  throw new Error('Usage: capture-performance-panel.mjs EVIDENCE_DIRECTORY')
const output = resolve(directory)
const results = { captures: [] }
let browser
try {
  browser = await chromium.launch({
    executablePath: chromium.executablePath(),
    headless: true,
    args: ['--no-sandbox', '--remote-debugging-port=9222'],
  })
  const browserSession = await browser.newBrowserCDPSession()
  results.browserVersion = await browserSession.send('Browser.getVersion')
  const versionResponse = await fetch('http://127.0.0.1:9222/json/version')
  if (!versionResponse.ok)
    throw new Error('Chrome debugging metadata is unavailable')
  const version = await versionResponse.json()
  results.debuggingVersion = version
  for (const route of ['login', 'root']) {
    const page = await browser.newPage({
      viewport: { width: 1920, height: 1080 },
    })
    const session = await page.context().newCDPSession(page)
    const { targetInfo } = await session.send('Target.getTargetInfo')
    const targetsResponse = await fetch('http://127.0.0.1:9222/json/list')
    const targets = await targetsResponse.json()
    const target = targets.find((entry) => entry.id === targetInfo.targetId)
    const frontendUrl = `devtools://devtools/bundled/inspector.html?panel=timeline&ws=127.0.0.1:9222/devtools/page/${targetInfo.targetId}`
    const capture = {
      route,
      trace: `simulate-01-${route}-0.trace.json`,
      bundledFrontendUrl: frontendUrl,
      devtoolsFrontendUrl: target?.devtoolsFrontendUrl ?? null,
      remoteBase: target?.devtoolsFrontendUrl
        ? new URL(
            target.devtoolsFrontendUrl,
            'http://127.0.0.1:9222',
          ).searchParams.get('remoteBase')
        : null,
    }
    results.captures.push(capture)
    const devtools = await browser.newPage({
      viewport: { width: 1920, height: 1080 },
    })
    await devtools.goto(frontendUrl)
    const fileInput = devtools.locator('input[type="file"]').first()
    await fileInput.waitFor({ state: 'attached', timeout: 30000 })
    await fileInput.setInputFiles(join(output, capture.trace))
    // Chrome's TraceBounds service is also used when a user selects a time range.
    // https://chromium.googlesource.com/devtools/devtools-frontend/+/HEAD/front_end/panels/timeline/
    await devtools.waitForFunction(
      async () => {
        const bounds =
          await import('/bundled/services/trace_bounds/trace_bounds.js')
        return Boolean(bounds.TraceBounds.BoundsManager.instance().state())
      },
      null,
      { timeout: 60000 },
    )
    const report = JSON.parse(
      readFileSync(join(output, `simulate-01-${route}.report.json`), 'utf8'),
    )
    const metrics = report.audits.metrics.details.items[0]
    capture.observedWindow = {
      fcpMs: metrics.observedFirstContentfulPaint,
      lcpMs: metrics.observedLargestContentfulPaint,
      fcpTs: metrics.observedFirstContentfulPaintTs,
      lcpTs: metrics.observedLargestContentfulPaintTs,
    }
    const { fcpTs, lcpTs } = capture.observedWindow
    if (!Number.isFinite(fcpTs) || !Number.isFinite(lcpTs)) {
      throw new Error(
        'Observed trace timestamps are required for the Performance selection',
      )
    }
    // Keep context when both paints are the same frame. Simulated timings cannot select a trace.
    capture.displayWindow = { min: fcpTs - 50000, max: lcpTs + 50000 }
    await devtools.evaluate(async ({ min, max }) => {
      const bounds =
        await import('/bundled/services/trace_bounds/trace_bounds.js')
      bounds.TraceBounds.BoundsManager.instance().setTimelineVisibleWindow(
        { min, max, range: max - min },
        { ignoreMiniMapBounds: true, shouldAnimate: false },
      )
      await new Promise((done) =>
        requestAnimationFrame(() => requestAnimationFrame(done)),
      )
    }, capture.displayWindow)
    const trace = JSON.parse(readFileSync(join(output, capture.trace), 'utf8'))
    const mainThreads = trace.traceEvents.filter(
      (event) =>
        event.name === 'thread_name' && event.args?.name === 'CrRendererMain',
    )
    capture.longTasks = trace.traceEvents
      .filter(
        (event) =>
          /RunTask$/.test(event.name) &&
          event.dur >= 50000 &&
          event.ts <= lcpTs &&
          event.ts + event.dur >= fcpTs &&
          mainThreads.some(
            (thread) => thread.pid === event.pid && thread.tid === event.tid,
          ),
      )
      .map((event) => ({
        name: event.name,
        ts: event.ts,
        durationMs: event.dur / 1000,
        pid: event.pid,
        tid: event.tid,
      }))
    capture.screenshot = `devtools-performance-${route}.png`
    await devtools.screenshot({ path: join(output, capture.screenshot) })
    capture.status =
      'observed FCP-to-LCP range captured with 50 ms context on either side'
    await devtools.close()
    await page.close()
  }
} catch (error) {
  results.error = error.message
  // A failed panel capture must remain visible as a failed CI step, with reproducible metadata.
  throw error
} finally {
  writeFileSync(
    join(output, 'devtools-panel.json'),
    JSON.stringify(results, null, 2) + '\n',
  )
  await browser?.close()
}
