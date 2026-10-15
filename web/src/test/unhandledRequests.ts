// MSW's 'error' strategy only rejects the fetch; app code that turns that
// rejection into a handled error would hide the missing handler. Recording each
// request lets afterEach fail the test regardless of what the code catches.
const unhandled: string[] = []

export function recordUnhandledRequest(request: Request) {
  unhandled.push(`${request.method} ${request.url}`)
}

export function assertNoUnhandledRequests() {
  if (unhandled.length === 0) return
  const list = unhandled.splice(0).join('\n  ')
  throw new Error(`Requests without a MSW handler:\n  ${list}`)
}
