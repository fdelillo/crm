import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'
import { SettingsLinkList } from './SettingsLinkList'

// T-002-F103 (002/DD-F12, P-F3): a list with one link per item.
describe('SettingsLinkList', () => {
  it('renders a list with one named link per item pointing at its route', () => {
    render(
      <MemoryRouter>
        <SettingsLinkList
          items={[
            { to: '/settings/company', label: 'Datos de la empresa' },
            { to: '/settings/users', label: 'Usuarios' },
          ]}
        />
      </MemoryRouter>,
    )

    const list = screen.getByRole('list')
    expect(within(list).getAllByRole('listitem')).toHaveLength(2)
    expect(
      screen.getByRole('link', { name: 'Datos de la empresa' }),
    ).toHaveAttribute('href', '/settings/company')
    expect(screen.getByRole('link', { name: 'Usuarios' })).toHaveAttribute(
      'href',
      '/settings/users',
    )
  })
})
