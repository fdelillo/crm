// Module-level flag (not router state): it survives the RootLayout remount that
// follows leaving the error screen by any route (link, Back, Forward), but not a
// page reload, so an initial load never moves focus (INV-F10, ui.md §18).
let pending = false

export function markErrorScreenExit() {
  pending = true
}

export function consumeErrorScreenExit() {
  const was = pending
  pending = false
  return was
}

export function resetErrorScreenExit() {
  pending = false
}
