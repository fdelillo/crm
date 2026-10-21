import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { LiveRegion } from './LiveRegion'

describe('LiveRegion', () => {
  it('keeps one atomic status region mounted as its content changes', () => {
    const { rerender } = render(<LiveRegion />)
    const region = screen.getByRole('status')

    expect(region).toBeEmptyDOMElement()
    expect(region).toHaveAttribute('aria-atomic', 'true')

    rerender(<LiveRegion>Operación completada.</LiveRegion>)

    expect(screen.getByRole('status')).toBe(region)
    expect(region).toHaveTextContent('Operación completada.')
  })
})
