import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { ArchivedSection } from './ArchivedSection'

// T-002-F103 (002/§16.1, 002/§18): disclosure of archived items.
describe('ArchivedSection', () => {
  it('starts collapsed and expands with the button', async () => {
    render(
      <ArchivedSection label="Ver archivadas" count={2}>
        <ul>
          <li>Alquiler de equipos</li>
          <li>Viáticos</li>
        </ul>
      </ArchivedSection>,
    )
    const toggle = screen.getByRole('button', { name: 'Ver archivadas (2)' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('Viáticos')).not.toBeVisible()

    await userEvent.setup().click(toggle)

    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('Viáticos')).toBeVisible()
    expect(toggle).toHaveAttribute(
      'aria-controls',
      screen.getByText('Viáticos').closest('[id]')?.id,
    )
  })

  it('collapses again with the keyboard', async () => {
    render(
      <ArchivedSection label="Ver archivados" count={1}>
        <p>Cable</p>
      </ArchivedSection>,
    )
    const user = userEvent.setup()
    await user.tab()
    await user.keyboard('{Enter}')
    expect(screen.getByText('Cable')).toBeVisible()

    await user.keyboard('{Enter}')

    expect(
      screen.getByRole('button', { name: 'Ver archivados (1)' }),
    ).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('Cable')).not.toBeVisible()
  })

  it('renders nothing when there are no archived items', () => {
    const { container } = render(
      <ArchivedSection label="Ver archivadas" count={0}>
        <p>Nada</p>
      </ArchivedSection>,
    )

    expect(container).toBeEmptyDOMElement()
  })
})
