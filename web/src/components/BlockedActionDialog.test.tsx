import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { BlockedActionDialog } from './BlockedActionDialog'

// T-002-F103 (002/§16.1, C-05): explains why an action cannot run; one button, "Entendido".
function Harness() {
  const [open, setOpen] = useState(false)
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Eliminar
      </button>
      <BlockedActionDialog
        open={open}
        onOpenChange={setOpen}
        title="No se puede eliminar «Finalizado»"
        description="Tiene que quedar una etapa de cierre ganado."
      />
    </>
  )
}

describe('BlockedActionDialog', () => {
  it('is an alert dialog with its title and description and one focused button', async () => {
    render(<Harness />)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Eliminar' }))

    const dialog = await screen.findByRole('alertdialog', {
      name: 'No se puede eliminar «Finalizado»',
    })
    expect(dialog).toHaveAccessibleDescription(
      'Tiene que quedar una etapa de cierre ganado.',
    )
    const buttons = screen.getAllByRole('button', { hidden: false })
    const inside = buttons.filter((button) => dialog.contains(button))
    expect(inside).toHaveLength(1)
    expect(inside[0]).toHaveAccessibleName('Entendido')
    expect(inside[0]).toHaveFocus()
  })

  it('returns the focus to the button that opened it', async () => {
    render(<Harness />)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Eliminar' }))
    await user.click(await screen.findByRole('button', { name: 'Entendido' }))

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Eliminar' })).toHaveFocus()
  })

  it('closes with Escape and returns the focus too', async () => {
    render(<Harness />)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Eliminar' }))
    await screen.findByRole('alertdialog')
    await user.keyboard('{Escape}')

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Eliminar' })).toHaveFocus()
  })
})
