import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClientProvider, useIsMutating } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { beforeAll, describe, expect, it } from 'vitest'
import { queryKeys } from '@/api/queryKeys'
import {
  fourStages,
  serveStages,
  stageId,
  type StagesServerOptions,
} from '@/test/configuration'
import { gate } from '@/test/gate'
import { createTestQueryClient } from '@/test/queryClient'
import { renderApp } from '@/test/renderApp'
import {
  actionsTrigger,
  liveRegion,
  openActionsMenu,
  preloadStagesModules,
  runRowAction,
  stageOrder,
} from '@/test/stageScreen'
import { STAGE_MUTATION_KEY, useMoveStage, useUpdateStage } from './api'

const four = fourStages()
const id = (name: string) => stageId(name, four)

async function renderFour(options: StagesServerOptions = {}) {
  const stages = serveStages({ stages: fourStages(), ...options })
  const view = renderApp(['/settings/stages'])
  await screen.findByRole('heading', {
    level: 1,
    name: 'Etapas de los proyectos',
  })
  await screen.findByText('Presupuesto')
  return { ...view, stages, user: userEvent.setup() }
}

const move = (name: string, position: number) => ({
  call: `POST /stages/${id(name)}/move`,
  body: { position },
})
const moves = (stages: { writes: Array<{ call: string }> }) =>
  stages.writes.filter((write) => write.call.endsWith('/move'))

const itemNamed = (label: string) =>
  screen.getByRole('menuitem', { name: label })

// T-002-F202 (002/INV-F3, INV-F11, BR-F14, DD-F10, DD-F14, S-F7): stage writes leave one at a time,
// and no position is computed over a list that a pending write is about to change.
describe('Stage writes: queue and consecutive moves', () => {
  beforeAll(preloadStagesModules)

  it('disables every move while another one waits, and sends nothing when they are activated', async () => {
    const held = gate()
    const { user, stages } = await renderFour({ holdMoveResponse: held.opened })
    await runRowAction(user, 'Finalizado', 'Subir')
    await waitFor(() => expect(moves(stages)).toHaveLength(1))
    expect(stages.writes).toEqual([move('Finalizado', 3)])

    await openActionsMenu(user, 'En curso')
    expect(liveRegion()).toHaveTextContent('Moviendo «Finalizado»…')
    for (const label of ['Subir', 'Bajar', 'Mover a otra posición…'])
      expect(itemNamed(label)).toHaveAttribute('aria-disabled', 'true')
    for (const label of ['Editar', 'Eliminar'])
      expect(itemNamed(label)).not.toHaveAttribute('aria-disabled')
    // The menu takes the focus into itself, on its first enabled item, never on a disabled one.
    expect(itemNamed('Editar')).toHaveFocus()
    expect(
      screen.getByRole('button', {
        name: 'Acciones para En curso',
        hidden: true,
      }),
    ).not.toBeDisabled()

    await user.click(itemNamed('Subir'))
    expect(itemNamed('Subir')).toBeVisible()
    expect(stages.writes).toEqual([move('Finalizado', 3)])

    await user.keyboard('{Escape}')
    await waitFor(async () =>
      expect(await actionsTrigger('En curso')).toHaveFocus(),
    )
    held.open()
    await waitFor(() =>
      expect(liveRegion()).toHaveTextContent(
        '«Finalizado» quedó en la posición 3 de 4.',
      ),
    )
  })

  it('computes the second move with the confirmed list and leaves the focus where the user put it', async () => {
    const held = gate()
    const { user, stages } = await renderFour({ holdMoveResponse: held.opened })
    await runRowAction(user, 'Finalizado', 'Subir')
    await waitFor(() => expect(moves(stages)).toHaveLength(1))
    await openActionsMenu(user, 'En curso')
    await user.click(itemNamed('Subir'))
    await user.keyboard('{Escape}')
    const trigger = await actionsTrigger('En curso')
    await waitFor(() => expect(trigger).toHaveFocus())

    held.open()
    await waitFor(() =>
      expect(stageOrder()).toEqual([
        'Presupuesto',
        'Aprobado',
        'Finalizado',
        'En curso',
      ]),
    )
    expect(liveRegion()).toHaveTextContent(
      '«Finalizado» quedó en la posición 3 de 4.',
    )
    // The focus does not jump to the moved row: it stays where the Escape left it.
    expect(trigger).toHaveFocus()
    await openActionsMenu(user, 'Presupuesto')
    for (const label of ['Bajar', 'Mover a otra posición…'])
      expect(itemNamed(label)).not.toHaveAttribute('aria-disabled')
    await user.keyboard('{Escape}')

    await runRowAction(user, 'En curso', 'Subir')
    await waitFor(() =>
      expect(stageOrder()).toEqual([
        'Presupuesto',
        'Aprobado',
        'En curso',
        'Finalizado',
      ]),
    )
    expect(stages.writes).toEqual([move('Finalizado', 3), move('En curso', 3)])
    expect(liveRegion()).toHaveTextContent(
      '«En curso» quedó en la posición 3 de 4.',
    )
  })

  it('blocks "Agregar etapa" while a move waits, keeps what was typed and sends the position of the new list', async () => {
    const held = gate()
    const { user, stages } = await renderFour({ holdMoveResponse: held.opened })
    await runRowAction(user, 'Finalizado', 'Subir')
    await waitFor(() => expect(moves(stages)).toHaveLength(1))

    await user.click(
      await screen.findByRole('button', { name: '+ Agregar etapa' }),
    )
    await user.type(
      await screen.findByRole('textbox', { name: 'Nombre' }),
      'Instalación',
    )
    const submit = screen.getByRole('button', { name: 'Agregar etapa' })
    expect(submit).toBeDisabled()
    expect(submit).toHaveAccessibleDescription(
      'Esperá a que termine el cambio de orden.',
    )
    expect(screen.getByRole('textbox', { name: 'Nombre' })).toHaveValue(
      'Instalación',
    )
    expect(
      stages.writes.filter((write) => write.call === 'POST /stages'),
    ).toEqual([])

    held.open()
    await waitFor(() => expect(submit).toBeEnabled())
    expect(submit).not.toHaveAccessibleDescription(
      'Esperá a que termine el cambio de orden.',
    )
    await user.click(submit)
    await screen.findByText('Agregaste la etapa «Instalación».')
    // Finalizado is now third: the stage goes before it, in position 3.
    expect(stages.writes.at(-1)).toEqual({
      call: 'POST /stages',
      body: { name: 'Instalación', type: 'open', color: 'gray', position: 3 },
    })
  })

  it('sends an edit made during a move only after the move was answered', async () => {
    const held = gate()
    const { user, stages } = await renderFour({ holdMoveResponse: held.opened })
    await runRowAction(user, 'Finalizado', 'Subir')
    await waitFor(() => expect(moves(stages)).toHaveLength(1))

    await runRowAction(user, 'Aprobado', 'Editar')
    const name = await screen.findByRole('textbox', { name: 'Nombre' })
    await user.clear(name)
    await user.type(name, 'Aceptado')
    await user.click(screen.getByRole('button', { name: 'Guardar etapa' }))
    // Waiting in the queue: the PATCH has not left.
    await screen.findByRole('button', { name: 'Guardando…' })
    expect(stages.writes).toEqual([move('Finalizado', 3)])

    held.open()
    await screen.findByText('Guardamos la etapa «Aceptado».')
    expect(stages.timeline).toEqual([
      `POST /stages/${id('Finalizado')}/move`,
      'move answered',
      `PATCH /stages/${id('Aprobado')}`,
    ])
    expect(stageOrder()).toEqual([
      'Presupuesto',
      'Aceptado',
      'Finalizado',
      'En curso',
    ])
  })

  it('keeps the answer of a move when a read that left before it answers after it', async () => {
    const heldRead = gate()
    const { user, stages, queryClient } = await renderFour({
      holdReadResponse: heldRead.opened,
    })
    // A read in flight, like the one a tab focus starts: it already took its snapshot of the old list.
    await act(async () => {
      void queryClient.refetchQueries({ queryKey: queryKeys.settings.stages })
    })
    await waitFor(() => expect(stages.reads).toBe(2))

    await runRowAction(user, 'Finalizado', 'Subir')
    await waitFor(() =>
      expect(liveRegion()).toHaveTextContent(
        '«Finalizado» quedó en la posición 3 de 4.',
      ),
    )
    heldRead.open()
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50))
    })
    expect(stageOrder()).toEqual([
      'Presupuesto',
      'Aprobado',
      'Finalizado',
      'En curso',
    ])
  })

  it('counts a mutation that waits in the queue (S-F7)', async () => {
    const held = gate()
    serveStages({ stages: fourStages(), holdMoveResponse: held.opened })
    const client = createTestQueryClient()
    let counted = -1
    function Probe() {
      counted = useIsMutating({ mutationKey: STAGE_MUTATION_KEY })
      const moveStage = useMoveStage()
      const updateStage = useUpdateStage()
      return (
        <>
          <button
            onClick={() => {
              moveStage.mutate({ stageId: id('Finalizado'), position: 3 })
              updateStage.mutate({
                stageId: id('Aprobado'),
                changes: { name: 'Aceptado' },
              })
            }}
          >
            Escribir
          </button>
          <p>
            {moveStage.isSuccess && updateStage.isSuccess
              ? 'Terminó'
              : 'Esperando'}
          </p>
        </>
      )
    }
    render(
      <QueryClientProvider client={client}>
        <Probe />
      </QueryClientProvider>,
    )
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Escribir' }))
    // One is on the wire and the other waits behind it: both count.
    await waitFor(() => expect(counted).toBe(2))
    held.open()
    await screen.findByText('Terminó')
    await waitFor(() => expect(counted).toBe(0))
  })

  it('shows the pending text only in the row being written', async () => {
    const held = gate()
    const { user } = await renderFour({ holdMoveResponse: held.opened })
    await runRowAction(user, 'Finalizado', 'Subir')
    await waitFor(() =>
      expect(
        within(
          screen.getByText('Finalizado', { selector: 'p' }).closest('li')!,
        ).getByText(/Moviendo…/),
      ).toBeVisible(),
    )
    expect(screen.getAllByText(/Moviendo…/)).toHaveLength(1)
    held.open()
  })
})
