import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { EmptyState, type EmptyStateProps } from './EmptyState'

const disabledLinkAction: EmptyStateProps['action'] = {
  label: 'Invitar',
  to: '/settings/users/invite',
  disabled: true,
  ariaDescribedBy: 'verification-notice',
}
const disabledButtonAction: EmptyStateProps['action'] = {
  label: 'Reintentar',
  onClick: () => undefined,
  disabled: true,
  ariaDescribedBy: 'retry-description',
}
// @ts-expect-error An action must navigate or provide an onClick callback.
const actionWithoutBehavior: EmptyStateProps['action'] = { label: 'Invitar' }
void disabledButtonAction
void actionWithoutBehavior

describe('EmptyState', () => {
  it('renders a disabled link action as a button described by its notice', () => {
    render(
      <EmptyState
        title="Todavía sos el único usuario."
        action={disabledLinkAction}
      />,
    )

    expect(screen.getByRole('button', { name: 'Invitar' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Invitar' })).toHaveAttribute(
      'aria-describedby',
      'verification-notice',
    )
    expect(
      screen.queryByRole('link', { name: 'Invitar' }),
    ).not.toBeInTheDocument()
  })
})
