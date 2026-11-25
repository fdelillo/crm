import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ReadOnlyNotice } from './ReadOnlyNotice'

// T-002-F103 (002/BR-F1): informative text, never announced on its own.
describe('ReadOnlyNotice', () => {
  it('shows the text without status or alert roles', () => {
    render(
      <ReadOnlyNotice text="Solo un administrador puede cambiar las etapas." />,
    )

    expect(
      screen.getByText('Solo un administrador puede cambiar las etapas.'),
    ).toBeVisible()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
