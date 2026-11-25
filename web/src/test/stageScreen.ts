import { screen, within } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'

// T-002-F202: what the stage tests do with the keyboard and the live region, written once.

export async function preloadStagesModules() {
  await Promise.all([
    import('@/features/auth/pages'),
    import('@/features/configuration/stages/StagesPage'),
    import('@/app/SettingsPage'),
    import('@/app/AppShell'),
  ])
}

export const actionsTrigger = (name: string) =>
  screen.findByRole('button', { name: `Acciones para ${name}` })

/** Focus the row's [⋮] and open its menu with Enter, like a keyboard user. */
export async function openActionsMenu(user: UserEvent, name: string) {
  const trigger = await actionsTrigger(name)
  trigger.focus()
  await user.keyboard('{Enter}')
  return screen.findByRole('menu')
}

/** Arrow down to the menu item with that text and press Enter. */
export async function pickMenuItem(user: UserEvent, label: string) {
  for (let step = 0; step < 6; step += 1) {
    const active = document.activeElement
    if (
      active?.getAttribute('role') === 'menuitem' &&
      active.textContent === label
    )
      break
    await user.keyboard('{ArrowDown}')
  }
  await user.keyboard('{Enter}')
}

export async function runRowAction(
  user: UserEvent,
  name: string,
  label: string,
) {
  await openActionsMenu(user, name)
  await pickMenuItem(user, label)
}

/** The names of the rows, in the order the user sees them. */
export const stageOrder = () =>
  screen
    .getAllByRole('button', { name: /^Acciones para / })
    .map((button) =>
      button.getAttribute('aria-label')?.replace('Acciones para ', ''),
    )

/** The live region of the screen: the status inside <main> (the banners of the shell have their own). */
export const liveRegion = () => {
  const main = screen.getByRole('main', { hidden: true })
  return within(main).getByRole('status', { hidden: true })
}
