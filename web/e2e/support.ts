import AxeBuilder from '@axe-core/playwright'
import { expect } from '@playwright/test'
import type { ConsoleMessage, Page } from '@playwright/test'

const mailpitURL = process.env.MAILPIT_URL ?? 'http://localhost:8025'

type MailpitSummary = {
  ID: string
  Subject: string
  To?: { Address: string }[]
}

type MailpitMessage = MailpitSummary & {
  Text: string
  HTML: string
}

export type BrowserDiagnostics = {
  consoleErrors: string[]
  pageErrors: string[]
  policyViolations: string[]
}

// T-F702 (NFR-F10): the browser itself logs "Failed to load resource" for every 4xx, including the
// ones the contract expects (anonymous /me, wrong password, duplicate email, reused link). Only
// those same-origin API rejections are left out; 5xx and anything the application logs still count.
const apiRejectionLog =
  /^Failed to load resource: the server responded with a status of 4\d\d\b/

function isExpectedAPIRejection(page: Page, message: ConsoleMessage): boolean {
  if (!apiRejectionLog.test(message.text())) return false
  try {
    const resource = new URL(message.location().url)
    return (
      resource.origin === new URL(page.url()).origin &&
      resource.pathname.startsWith('/api/v1/')
    )
  } catch {
    return false
  }
}

export async function captureBrowserDiagnostics(
  page: Page,
): Promise<BrowserDiagnostics> {
  const diagnostics: BrowserDiagnostics = {
    consoleErrors: [],
    pageErrors: [],
    policyViolations: [],
  }

  page.on('console', (message) => {
    if (message.type() !== 'error' || isExpectedAPIRejection(page, message))
      return
    diagnostics.consoleErrors.push(message.text())
  })
  page.on('pageerror', (error) => diagnostics.pageErrors.push(error.message))
  await page.exposeFunction('__crmReportCSPViolation', (detail: string) => {
    diagnostics.policyViolations.push(detail)
  })
  await page.addInitScript(() => {
    document.addEventListener('securitypolicyviolation', (event) => {
      const target = window as Window & {
        __crmReportCSPViolation?: (detail: string) => void
      }
      void target.__crmReportCSPViolation?.(
        `${event.violatedDirective}: ${event.blockedURI}`,
      )
    })
  })
  return diagnostics
}

export async function expectNoAccessibilityViolations(
  page: Page,
): Promise<void> {
  // Axe measures contrast mid-animation, so wait until no toast is leaving and every visible one is
  // opaque. A single polled check: a toast can start leaving between counting and inspecting it.
  await expect
    .poll(
      () =>
        page.evaluate(
          () =>
            !document.querySelector(
              '[data-sonner-toast][data-removed="true"]',
            ) &&
            Array.from(
              document.querySelectorAll<HTMLElement>(
                '[data-sonner-toast][data-mounted="true"][data-visible="true"]',
              ),
            ).every((toast) => getComputedStyle(toast).opacity === '1'),
        ),
      { message: 'toasts settled before the accessibility scan' },
    )
    .toBe(true)

  const result = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze()

  if (result.violations.length > 0) {
    const details = result.violations
      .map(
        (violation) =>
          `${violation.id} (${violation.impact ?? 'sin impacto'}): ${violation.help}\n${violation.nodes
            .map(
              (node) =>
                `  ${node.target.join(' ')} (${node.html}) — ${node.failureSummary ?? node.any.map((check) => check.message).join('; ')}`,
            )
            .join('\n')}`,
      )
      .join('\n')
    throw new Error(
      `Se encontraron violaciones de accesibilidad WCAG 2.x AA:\n${details}`,
    )
  }
}

export async function readLatestEmail(
  subject?: string,
  recipient?: string,
  excludedID?: string,
): Promise<MailpitMessage> {
  const response = await fetch(`${mailpitURL}/api/v1/messages?limit=50`)
  if (!response.ok)
    throw new Error(`Mailpit respondió ${response.status} al listar mensajes`)

  const result = (await response.json()) as { messages?: MailpitSummary[] }
  const message = result.messages?.find(
    (candidate) =>
      candidate.ID !== excludedID &&
      (!subject || candidate.Subject === subject) &&
      (!recipient ||
        candidate.To?.some(
          ({ Address }) => Address.toLowerCase() === recipient.toLowerCase(),
        )),
  )
  if (!message)
    throw new Error(
      `No se encontró un email${subject ? ` con asunto "${subject}"` : ''}${recipient ? ` para ${recipient}` : ''}`,
    )

  const detailResponse = await fetch(
    `${mailpitURL}/api/v1/message/${encodeURIComponent(message.ID)}`,
  )
  if (!detailResponse.ok)
    throw new Error(
      `Mailpit respondió ${detailResponse.status} al leer el mensaje`,
    )
  return (await detailResponse.json()) as MailpitMessage
}

export async function waitForEmail(
  subject: string,
  recipient: string,
  excludedID?: string,
): Promise<MailpitMessage> {
  let latest: MailpitMessage | null = null
  await expect
    .poll(
      async () => {
        try {
          latest = await readLatestEmail(subject, recipient, excludedID)
          return latest.ID
        } catch {
          return null
        }
      },
      {
        timeout: 20_000,
        message: `esperando el email ${subject} para ${recipient}`,
      },
    )
    .not.toBeNull()
  if (!latest) throw new Error(`No llegó el email ${subject} para ${recipient}`)
  return latest
}

export function extractEmailLink(message: MailpitMessage): URL {
  const content = `${message.HTML}\n${message.Text}`
  const candidate = content.match(
    /https:\/\/localhost:8443\/[\w/?#=&.%+-]+/,
  )?.[0]
  if (!candidate)
    throw new Error('El email no contiene un enlace HTTPS a localhost:8443')
  return new URL(candidate)
}
