import { useRef, useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { FormDialog, type FormDialogProps } from './FormDialog'

// T-002-F103 (002/§16.1, 002/§18): the shared form dialog. Opened by a real button, like a page does.
type Extra = Partial<Omit<FormDialogProps, 'open' | 'onOpenChange'>>

function Harness({
  onSubmit = (event) => event.preventDefault(),
  onClose,
  ...extra
}: Extra & { onClose?: () => void }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Abrir
      </button>
      <FormDialog
        open={open}
        onOpenChange={(next) => {
          setOpen(next)
          if (!next) onClose?.()
        }}
        title="Nueva etapa"
        description="Se agrega a la lista."
        submitLabel="Agregar etapa"
        pendingLabel="Agregando…"
        pending={false}
        onSubmit={onSubmit}
        {...extra}
      >
        <label>
          Nombre
          <input name="name" />
        </label>
        <label>
          Notas
          <input name="notes" />
        </label>
      </FormDialog>
    </>
  )
}

async function openDialog() {
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Abrir' }))
  return user
}

describe('FormDialog', () => {
  it('is a dialog named by its title with the focus on the first field', async () => {
    render(<Harness />)
    await openDialog()

    const dialog = await screen.findByRole('dialog', { name: 'Nueva etapa' })
    expect(dialog).toHaveAccessibleDescription('Se agrega a la lista.')
    expect(screen.getByRole('textbox', { name: 'Nombre' })).toHaveFocus()
  })

  it('closes with Escape and returns the focus to the button that opened it', async () => {
    const onClose = vi.fn()
    render(<Harness onClose={onClose} />)
    const user = await openDialog()
    await screen.findByRole('dialog')

    await user.keyboard('{Escape}')

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: 'Abrir' })).toHaveFocus()
  })

  it('closes with Cancelar', async () => {
    render(<Harness />)
    const user = await openDialog()
    await user.click(await screen.findByRole('button', { name: 'Cancelar' }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Abrir' })).toHaveFocus()
  })

  it('sends the form with the submit button', async () => {
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault())
    render(<Harness onSubmit={onSubmit} />)
    const user = await openDialog()
    await user.click(
      await screen.findByRole('button', { name: 'Agregar etapa' }),
    )

    expect(onSubmit).toHaveBeenCalledTimes(1)
  })

  it('focuses the element the page chooses instead of the first field', async () => {
    function WithInitialFocus() {
      const [open, setOpen] = useState(false)
      const notes = useRef<HTMLInputElement>(null)
      return (
        <>
          <button type="button" onClick={() => setOpen(true)}>
            Abrir
          </button>
          <FormDialog
            open={open}
            onOpenChange={setOpen}
            title="Nueva etapa"
            submitLabel="Agregar etapa"
            pendingLabel="Agregando…"
            pending={false}
            onSubmit={(event) => event.preventDefault()}
            initialFocusRef={notes}
          >
            <input aria-label="Nombre" />
            <input aria-label="Notas" ref={notes} />
          </FormDialog>
        </>
      )
    }
    render(<WithInitialFocus />)
    await openDialog()

    expect(await screen.findByRole('textbox', { name: 'Notas' })).toHaveFocus()
  })

  describe('while pending', () => {
    it('disables cancel and submit, shows the pending label and ignores Escape', async () => {
      const onClose = vi.fn()
      render(<Harness pending onClose={onClose} />)
      const user = await openDialog()
      await screen.findByRole('dialog')

      expect(screen.getByRole('button', { name: 'Cancelar' })).toBeDisabled()
      expect(screen.getByRole('button', { name: 'Agregando…' })).toBeDisabled()
      expect(
        screen.queryByRole('button', { name: 'Agregar etapa' }),
      ).not.toBeInTheDocument()

      await user.keyboard('{Escape}')

      expect(screen.getByRole('dialog')).toBeInTheDocument()
      expect(onClose).not.toHaveBeenCalled()
    })
  })

  describe('with a submitBlockedReason', () => {
    const reason = 'Esperá a que termine el movimiento en curso.'

    it('disables the submit button and describes it with the visible reason', async () => {
      render(<Harness submitBlockedReason={reason} />)
      await openDialog()

      const submit = await screen.findByRole('button', {
        name: 'Agregar etapa',
      })
      expect(submit).toBeDisabled()
      expect(screen.getByText(reason)).toBeVisible()
      expect(submit).toHaveAccessibleDescription(reason)
    })

    it('keeps Cancelar and Escape working and keeps what was typed', async () => {
      const onClose = vi.fn()
      const { rerender } = render(
        <Harness submitBlockedReason={reason} onClose={onClose} />,
      )
      const user = await openDialog()
      const name = await screen.findByRole('textbox', { name: 'Nombre' })
      await user.type(name, 'Instalación')

      expect(screen.getByRole('button', { name: 'Cancelar' })).toBeEnabled()

      // The reason goes away while the dialog stays open: same field value, submit enabled.
      rerender(<Harness onClose={onClose} />)
      expect(screen.getByRole('textbox', { name: 'Nombre' })).toHaveValue(
        'Instalación',
      )
      expect(
        screen.getByRole('button', { name: 'Agregar etapa' }),
      ).toBeEnabled()
      expect(screen.queryByText(reason)).not.toBeInTheDocument()
    })

    it('closes with Escape while the submit is blocked', async () => {
      const onClose = vi.fn()
      render(<Harness submitBlockedReason={reason} onClose={onClose} />)
      const user = await openDialog()
      await screen.findByRole('dialog')

      await user.keyboard('{Escape}')

      expect(onClose).toHaveBeenCalledTimes(1)
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })

    it('closes with Cancelar while the submit is blocked', async () => {
      render(<Harness submitBlockedReason={reason} />)
      const user = await openDialog()
      await user.click(await screen.findByRole('button', { name: 'Cancelar' }))

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
  })
})
