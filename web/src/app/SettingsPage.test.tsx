import { screen, within } from '@testing-library/react'
import { beforeAll, describe, expect, it } from 'vitest'
import {
  adminSession,
  operatorSession,
  serveSession,
} from '@/test/configuration'
import { renderApp } from '@/test/renderApp'
import { preloadStagesModules } from '@/test/stageScreen'

async function renderSettings() {
  renderApp(['/settings'])
  await screen.findByRole('heading', { level: 1, name: 'Ajustes' })
}

// T-002-F202 (002/ui.md §14.1, FR-008, DD-F12): the "Configuración" section is for every role.
describe('Settings page: Configuración section', () => {
  beforeAll(preloadStagesModules)

  it('links the administrator to the stages screen', async () => {
    serveSession(adminSession())
    await renderSettings()
    const section = screen
      .getByRole('heading', { level: 2, name: 'Configuración' })
      .closest('section')
    expect(section).not.toBeNull()
    expect(
      within(section!).getByRole('link', { name: /Etapas de los proyectos/ }),
    ).toHaveAttribute('href', '/settings/stages')
    expect(
      screen.queryByText('Podés consultarla; solo un administrador la cambia.'),
    ).toBeNull()
    expect(
      screen.getByRole('heading', { level: 2, name: 'Tu empresa' }),
    ).toBeVisible()
  })

  it('shows the operator the same link with a note that only an administrator changes it', async () => {
    serveSession(operatorSession())
    await renderSettings()
    expect(
      screen.getByRole('link', { name: /Etapas de los proyectos/ }),
    ).toBeVisible()
    expect(
      screen.getByText('Podés consultarla; solo un administrador la cambia.'),
    ).toBeVisible()
    expect(screen.queryByRole('heading', { name: 'Tu empresa' })).toBeNull()
  })
})
