import { screen, waitFor, within } from '@testing-library/react'
import userEvent, { type UserEvent } from '@testing-library/user-event'
import { http, HttpResponse, delay } from 'msw'
import { beforeAll, describe, expect, it } from 'vitest'
import { base, problemResponse } from '@/test/company'
import {
  adminSession,
  genericStages,
  operatorSession,
  serveStages,
  stageId,
  type StagesServerOptions,
} from '@/test/configuration'
import { gate } from '@/test/gate'
import { server } from '@/test/msw/server'
import { renderApp } from '@/test/renderApp'
import {
  actionsTrigger,
  liveRegion,
  openActionsMenu,
  pickMenuItem,
  preloadStagesModules,
  runRowAction,
  stageOrder,
} from '@/test/stageScreen'

async function renderStages(options?: StagesServerOptions) {
  const stages = serveStages(options)
  const view = renderApp(['/settings/stages'])
  await screen.findByRole('heading', {
    level: 1,
    name: 'Etapas de los proyectos',
  })
  await screen.findByText('Presupuesto')
  return { ...view, stages, user: userEvent.setup() }
}

const dialog = (name: string) => screen.findByRole('dialog', { name })
const row = (name: string) => {
  const item = screen.getByText(name, { selector: 'p' }).closest('li')
  if (!item) throw new Error(`No row for ${name}`)
  return item
}
const addButton = () => screen.findByRole('button', { name: '+ Agregar etapa' })

async function openCreateDialog(user: UserEvent) {
  await user.click(await addButton())
  return dialog('Nueva etapa')
}

async function createStage(user: UserEvent, name: string) {
  await openCreateDialog(user)
  await user.type(screen.getByRole('textbox', { name: 'Nombre' }), name)
  await user.click(screen.getByRole('button', { name: 'Agregar etapa' }))
}

// T-002-F202 (Historia 1, FR-001, FR-008, BR-F1 a BR-F5, BR-F12, INV-F6, INV-F8, DD-F1, DD-F5, DD-F6)
describe('StagesPage', () => {
  beforeAll(preloadStagesModules)

  describe('reading', () => {
    it('shows a busy region while the stages load', async () => {
      serveStages()
      server.use(http.get(`${base}/stages`, () => delay('infinite')))
      renderApp(['/settings/stages'])
      const loading = await screen.findByRole('region', {
        name: 'Cargando las etapas',
      })
      expect(loading).toHaveAttribute('aria-busy', 'true')
    })

    it('lists the five stages in order with their type in text', async () => {
      await renderStages()
      expect(stageOrder()).toEqual([
        'Presupuesto',
        'Aprobado',
        'En curso',
        'Finalizado',
        'Perdido',
      ])
      const list = screen.getByRole('list')
      expect(within(list).getAllByRole('listitem')).toHaveLength(5)
      expect(row('Presupuesto')).toHaveTextContent('Abierta · Etapa inicial')
      expect(row('Aprobado')).toHaveTextContent(/^Aprobado\s*Abierta$/)
      expect(row('Finalizado')).toHaveTextContent('Cierre ganado')
      expect(row('Perdido')).toHaveTextContent('Cierre perdido')
    })

    it('says so when the list is empty and lets the administrator add', async () => {
      serveStages({ stages: [] })
      renderApp(['/settings/stages'])
      expect(await screen.findByText('Todavía no hay etapas.')).toBeVisible()
      expect(
        await screen.findByRole('button', { name: '+ Agregar etapa' }),
      ).toBeVisible()
    })

    it('offers to retry with the error code when the list cannot be read', async () => {
      serveStages()
      server.use(
        http.get(`${base}/stages`, () =>
          problemResponse('internal', 500, { instance: 'req-123' }),
        ),
      )
      const user = userEvent.setup()
      renderApp(['/settings/stages'])
      expect(
        await screen.findByText('No pudimos cargar las etapas.'),
      ).toBeVisible()
      expect(screen.getAllByText(/req-123/).length).toBeGreaterThan(0)
      serveStages()
      await user.click(screen.getByRole('button', { name: 'Reintentar' }))
      expect(await screen.findByText('Presupuesto')).toBeVisible()
    })

    it('shows the operator a read-only list with a notice and never "Sin permiso"', async () => {
      serveStages({ session: operatorSession() })
      renderApp(['/settings/stages'])
      expect(await screen.findByText('Presupuesto')).toBeVisible()
      expect(
        screen.getByText('Solo un administrador puede cambiar las etapas.'),
      ).toBeVisible()
      expect(
        screen.queryByRole('button', { name: '+ Agregar etapa' }),
      ).toBeNull()
      expect(
        screen.queryByRole('button', { name: /^Acciones para / }),
      ).toBeNull()
      expect(screen.queryByText(/Sin permiso/)).toBeNull()
    })
  })

  describe('the actions menu', () => {
    it('has no "Subir" on the first stage and no "Bajar" on the last, reached with the keyboard', async () => {
      const { user } = await renderStages()
      await openActionsMenu(user, 'Presupuesto')
      expect(
        screen.getAllByRole('menuitem').map((item) => item.textContent),
      ).toEqual(['Editar', 'Bajar', 'Mover a otra posición…', 'Eliminar'])
      await user.keyboard('{Escape}')
      await openActionsMenu(user, 'Perdido')
      expect(
        screen.getAllByRole('menuitem').map((item) => item.textContent),
      ).toEqual(['Editar', 'Subir', 'Mover a otra posición…', 'Eliminar'])
    })

    it('offers every action to a stage in the middle', async () => {
      const { user } = await renderStages()
      await openActionsMenu(user, 'En curso')
      expect(
        screen.getAllByRole('menuitem').map((item) => item.textContent),
      ).toEqual([
        'Editar',
        'Subir',
        'Bajar',
        'Mover a otra posición…',
        'Eliminar',
      ])
    })
  })

  describe('moving', () => {
    it('moves a stage up, announces it and leaves the focus on its menu', async () => {
      const held = gate()
      const { user, stages } = await renderStages({
        holdMoveResponse: held.opened,
      })
      expect(liveRegion()).toBeEmptyDOMElement()
      await runRowAction(user, 'En curso', 'Subir')
      await waitFor(() =>
        expect(liveRegion()).toHaveTextContent('Moviendo «En curso»…'),
      )
      expect(row('En curso')).toHaveTextContent('Moviendo…')
      held.open()
      await waitFor(() =>
        expect(liveRegion()).toHaveTextContent(
          '«En curso» quedó en la posición 2 de 5.',
        ),
      )
      expect(stageOrder()).toEqual([
        'Presupuesto',
        'En curso',
        'Aprobado',
        'Finalizado',
        'Perdido',
      ])
      expect(row('En curso')).not.toHaveTextContent('Moviendo…')
      expect(stages.writes).toEqual([
        {
          call: `POST /stages/${stageId('En curso')}/move`,
          body: { position: 2 },
        },
      ])
      await waitFor(async () =>
        expect(await actionsTrigger('En curso')).toHaveFocus(),
      )
    })

    it('explains instead of moving when the stage would end up first', async () => {
      const { user, stages } = await renderStages()
      for (const expected of [3, 2, 1] as const) {
        await runRowAction(user, 'Finalizado', 'Subir')
        if (expected === 1) break
        await waitFor(() =>
          expect(liveRegion()).toHaveTextContent(`posición ${expected} de 5`),
        )
      }
      const blocked = await screen.findByRole('alertdialog', {
        name: 'No se puede mover «Finalizado» ahí',
      })
      expect(blocked).toHaveTextContent(
        'La primera etapa tiene que ser abierta: es donde empiezan los proyectos nuevos.',
      )
      expect(stages.writes).toHaveLength(2)
      const understood = within(blocked).getByRole('button', {
        name: 'Entendido',
      })
      expect(understood).toHaveFocus()
      await user.keyboard('{Enter}')
      await waitFor(async () =>
        expect(await actionsTrigger('Finalizado')).toHaveFocus(),
      )
    })

    it('moves to the chosen position from "Mover a otra posición…"', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'En curso', 'Mover a otra posición…')
      await dialog('Mover «En curso»')
      const select = screen.getByRole('combobox', { name: 'Posición' })
      expect(select).toHaveTextContent(
        '3 (después de «Aprobado») (lugar actual)',
      )
      await user.click(select)
      await user.click(
        await screen.findByRole('option', { name: '1 (al principio)' }),
      )
      await user.click(screen.getByRole('button', { name: 'Mover' }))
      await waitFor(() =>
        expect(liveRegion()).toHaveTextContent(
          '«En curso» quedó en la posición 1 de 5.',
        ),
      )
      expect(stages.writes).toEqual([
        {
          call: `POST /stages/${stageId('En curso')}/move`,
          body: { position: 1 },
        },
      ])
      expect(stageOrder()[0]).toBe('En curso')
      await waitFor(async () =>
        expect(await actionsTrigger('En curso')).toHaveFocus(),
      )
    })

    it('closes "Mover a otra posición…" without a request when the position is the same', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'En curso', 'Mover a otra posición…')
      await dialog('Mover «En curso»')
      await user.click(screen.getByRole('button', { name: 'Mover' }))
      await waitFor(() =>
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
      )
      expect(stages.writes).toEqual([])
      await waitFor(async () =>
        expect(await actionsTrigger('En curso')).toHaveFocus(),
      )
    })

    it('shows the rule as an error in the field when the chosen position breaks it', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'Finalizado', 'Mover a otra posición…')
      await dialog('Mover «Finalizado»')
      await user.click(screen.getByRole('combobox', { name: 'Posición' }))
      await user.click(
        await screen.findByRole('option', { name: '1 (al principio)' }),
      )
      await user.click(screen.getByRole('button', { name: 'Mover' }))
      expect(
        await screen.findByText(
          'La primera etapa tiene que ser abierta: es donde empiezan los proyectos nuevos.',
        ),
      ).toBeVisible()
      expect(stages.writes).toEqual([])
    })

    it('says the stage is gone when the move answers 404 and reads the list again', async () => {
      const { user, stages } = await renderStages()
      server.use(
        http.post(`${base}/stages/:stageId/move`, () =>
          problemResponse('not_found', 404),
        ),
      )
      await runRowAction(user, 'En curso', 'Subir')
      expect(
        await screen.findByText(
          'Esa etapa ya no existe. Actualizamos la lista.',
        ),
      ).toBeVisible()
      await waitFor(() => expect(stages.reads).toBe(2))
    })
  })

  describe('deleting', () => {
    it('explains why the only "Cierre ganado" stage cannot be deleted', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'Finalizado', 'Eliminar')
      const blocked = await screen.findByRole('alertdialog', {
        name: 'No se puede eliminar «Finalizado»',
      })
      expect(blocked).toHaveTextContent(
        'Tiene que quedar al menos una etapa de cierre ganado. Antes, marcá otra etapa como «Cierre ganado».',
      )
      expect(stages.writes).toEqual([])
      await user.keyboard('{Enter}')
      await waitFor(async () =>
        expect(await actionsTrigger('Finalizado')).toHaveFocus(),
      )
    })

    it('explains that the first stage would be a closing one', async () => {
      const { user, stages } = await renderStages({
        stages: genericStages().filter(
          (stage) => stage.name !== 'Aprobado' && stage.name !== 'En curso',
        ),
      })
      await runRowAction(user, 'Presupuesto', 'Eliminar')
      const blocked = await screen.findByRole('alertdialog', {
        name: 'No se puede eliminar «Presupuesto»',
      })
      expect(blocked).toHaveTextContent(
        'Después de eliminarla, la primera etapa sería «Finalizado», que es de cierre. La primera tiene que ser abierta: antes, mové una etapa abierta al principio.',
      )
      expect(stages.writes).toEqual([])
    })

    it('asks for confirmation, deletes and moves the focus to the next stage', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'Aprobado', 'Eliminar')
      const confirm = await screen.findByRole('alertdialog', {
        name: '¿Eliminar la etapa «Aprobado»?',
      })
      expect(confirm).toHaveTextContent(
        'Sale de la lista y no se puede recuperar. Si después la necesitás, creá una nueva.',
      )
      expect(
        within(confirm).getByRole('button', { name: 'Cancelar' }),
      ).toHaveFocus()
      await user.click(
        within(confirm).getByRole('button', { name: 'Eliminar' }),
      )
      expect(
        await screen.findByText('Eliminaste la etapa «Aprobado».'),
      ).toBeVisible()
      expect(stages.writes).toEqual([
        { call: `DELETE /stages/${stageId('Aprobado')}`, body: undefined },
      ])
      await waitFor(() => expect(stages.reads).toBe(2))
      expect(stageOrder()).toEqual([
        'Presupuesto',
        'En curso',
        'Finalizado',
        'Perdido',
      ])
      await waitFor(async () =>
        expect(await actionsTrigger('En curso')).toHaveFocus(),
      )
    })

    it('moves the focus to the previous stage when the last one is deleted', async () => {
      const { user } = await renderStages()
      await runRowAction(user, 'Perdido', 'Eliminar')
      await user.click(
        within(await screen.findByRole('alertdialog')).getByRole('button', {
          name: 'Eliminar',
        }),
      )
      expect(
        await screen.findByText('Eliminaste la etapa «Perdido».'),
      ).toBeVisible()
      await waitFor(async () =>
        expect(await actionsTrigger('Finalizado')).toHaveFocus(),
      )
    })

    // INV-F6, BR-F14: the neighbour is the one in the order the DELETE leaves with, not the one on screen when confirming.
    it('moves the focus to the neighbour in the order a held move left, not the one seen when confirming', async () => {
      const held = gate()
      const { user, stages } = await renderStages({
        holdMoveResponse: held.opened,
      })
      await runRowAction(user, 'Finalizado', 'Subir')
      await waitFor(() => expect(stages.writes).toHaveLength(1))
      await runRowAction(user, 'Aprobado', 'Eliminar')
      await user.click(
        within(await screen.findByRole('alertdialog')).getByRole('button', {
          name: 'Eliminar',
        }),
      )
      expect(stages.writes).toHaveLength(1)
      held.open()
      expect(
        await screen.findByText('Eliminaste la etapa «Aprobado».'),
      ).toBeVisible()
      expect(stageOrder()).toEqual([
        'Presupuesto',
        'Finalizado',
        'En curso',
        'Perdido',
      ])
      await waitFor(async () =>
        expect(await actionsTrigger('Finalizado')).toHaveFocus(),
      )
    })

    it('returns the focus to the row when the confirmation is cancelled', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'Aprobado', 'Eliminar')
      await screen.findByRole('alertdialog')
      await user.keyboard('{Escape}')
      await waitFor(async () =>
        expect(await actionsTrigger('Aprobado')).toHaveFocus(),
      )
      expect(stages.writes).toEqual([])
    })

    it('explains with the generic text and reads the list again when the server says invalid_state', async () => {
      const { user, stages } = await renderStages()
      server.use(
        http.delete(`${base}/stages/:stageId`, () =>
          problemResponse('invalid_state', 409),
        ),
      )
      await runRowAction(user, 'Aprobado', 'Eliminar')
      await user.click(
        within(await screen.findByRole('alertdialog')).getByRole('button', {
          name: 'Eliminar',
        }),
      )
      expect(
        await screen.findByText(
          /No pudimos hacer ese cambio en las etapas\. La primera etapa tiene que ser abierta/,
        ),
      ).toBeVisible()
      expect(screen.queryByRole('alertdialog')).toBeNull()
      await waitFor(() => expect(stages.reads).toBe(2))
    })

    it('says the stage is gone when delete answers 404', async () => {
      const { user, stages } = await renderStages()
      server.use(
        http.delete(`${base}/stages/:stageId`, () =>
          problemResponse('not_found', 404),
        ),
      )
      await runRowAction(user, 'Aprobado', 'Eliminar')
      await user.click(
        within(await screen.findByRole('alertdialog')).getByRole('button', {
          name: 'Eliminar',
        }),
      )
      expect(
        await screen.findByText(
          'Esa etapa ya no existe. Actualizamos la lista.',
        ),
      ).toBeVisible()
      await waitFor(() => expect(stages.reads).toBe(2))
    })
  })

  describe('adding', () => {
    it('opens with the name focused and the suggested type, color and place', async () => {
      const { user } = await renderStages()
      const form = await openCreateDialog(user)
      expect(
        within(form).getByRole('textbox', { name: 'Nombre' }),
      ).toHaveFocus()
      expect(within(form).getByRole('radio', { name: 'Abierta' })).toBeChecked()
      expect(within(form).getByRole('radio', { name: 'Naranja' })).toBeChecked()
      expect(
        within(form).getByRole('combobox', { name: 'Ubicación' }),
      ).toHaveTextContent('Antes de «Finalizado»')
      for (const color of [
        'Azul',
        'Verde azulado',
        'Verde',
        'Amarillo',
        'Naranja',
        'Rojo',
        'Violeta',
        'Gris',
      ])
        expect(within(form).getByRole('radio', { name: color })).toBeVisible()
    })

    it('adds the stage before the closing ones and gives the focus back to "+ Agregar etapa"', async () => {
      const { user, stages } = await renderStages()
      await createStage(user, 'Instalación')
      expect(
        await screen.findByText('Agregaste la etapa «Instalación».'),
      ).toBeVisible()
      expect(stages.writes).toEqual([
        {
          call: 'POST /stages',
          body: {
            name: 'Instalación',
            type: 'open',
            color: 'orange',
            position: 4,
          },
        },
      ])
      await waitFor(() => expect(stages.reads).toBe(2))
      expect(stageOrder()).toEqual([
        'Presupuesto',
        'Aprobado',
        'En curso',
        'Instalación',
        'Finalizado',
        'Perdido',
      ])
      await waitFor(async () => expect(await addButton()).toHaveFocus())
    })

    it('sends no position when "Al final" is chosen and trims the name', async () => {
      const { user, stages } = await renderStages()
      await openCreateDialog(user)
      await user.type(
        screen.getByRole('textbox', { name: 'Nombre' }),
        '  Entrega  ',
      )
      await user.click(screen.getByRole('combobox', { name: 'Ubicación' }))
      await user.click(await screen.findByRole('option', { name: 'Al final' }))
      await user.click(screen.getByRole('button', { name: 'Agregar etapa' }))
      await screen.findByText('Agregaste la etapa «Entrega».')
      expect(stages.writes[0]?.body).toEqual({
        name: 'Entrega',
        type: 'open',
        color: 'orange',
      })
    })

    it('rejects a name already in the list, ignoring case, without a request', async () => {
      const { user, stages } = await renderStages()
      await createStage(user, 'presupuesto')
      const name = screen.getByRole('textbox', { name: 'Nombre' })
      expect(
        await screen.findByText('Ya hay una etapa con ese nombre.'),
      ).toBeVisible()
      expect(name).toHaveAccessibleDescription(
        'Ya hay una etapa con ese nombre.',
      )
      expect(stages.writes).toEqual([])
    })

    it('asks for a name when it is only spaces, with the spaces Go trims', async () => {
      const { user, stages } = await renderStages()
      await createStage(user, ' \u0085 ')
      expect(await screen.findByText('Escribí un nombre.')).toBeVisible()
      expect(stages.writes).toEqual([])
    })

    it('rejects a "Cierre ganado" stage placed first', async () => {
      const { user, stages } = await renderStages()
      await openCreateDialog(user)
      await user.type(screen.getByRole('textbox', { name: 'Nombre' }), 'Cierre')
      await user.click(screen.getByRole('radio', { name: 'Cierre ganado' }))
      await user.click(screen.getByRole('combobox', { name: 'Ubicación' }))
      await user.click(
        await screen.findByRole('option', { name: 'Antes de «Presupuesto»' }),
      )
      await user.click(screen.getByRole('button', { name: 'Agregar etapa' }))
      const message =
        'La primera etapa tiene que ser abierta: es donde empiezan los proyectos nuevos.'
      expect(await screen.findByText(message)).toBeVisible()
      expect(
        screen.getByRole('radiogroup', { name: 'Tipo' }),
      ).toHaveAccessibleDescription(message)
      expect(stages.writes).toEqual([])
    })

    it('counts an emoji as one character: 61 are too many, 60 are fine', async () => {
      const { user, stages } = await renderStages()
      await openCreateDialog(user)
      const name = screen.getByRole('textbox', { name: 'Nombre' })
      await user.type(name, `${'a'.repeat(60)}😀`)
      await user.click(screen.getByRole('button', { name: 'Agregar etapa' }))
      expect(
        await screen.findByText('Es demasiado largo (máximo 60 caracteres).'),
      ).toBeVisible()
      expect(stages.writes).toEqual([])
      await user.clear(name)
      await user.type(name, `${'a'.repeat(59)}😀`)
      await user.click(screen.getByRole('button', { name: 'Agregar etapa' }))
      await waitFor(() => expect(stages.writes).toHaveLength(1))
    })

    it('marks the name as taken when the server says so, focuses it and reads the list again', async () => {
      const { user, stages } = await renderStages()
      server.use(
        http.post(`${base}/stages`, () => problemResponse('name_taken', 409)),
      )
      await createStage(user, 'Instalación')
      const error = await screen.findByText('Ya hay una etapa con ese nombre.')
      expect(error).toBeVisible()
      expect(screen.getByRole('textbox', { name: 'Nombre' })).toHaveFocus()
      await waitFor(() => expect(stages.reads).toBe(2))
    })

    it('keeps what was typed and shows the message of the server when it is down', async () => {
      const { user } = await renderStages()
      server.use(
        http.post(`${base}/stages`, () =>
          problemResponse('service_unavailable', 503),
        ),
      )
      await createStage(user, 'Instalación')
      const form = await dialog('Nueva etapa')
      expect(
        await within(form).findByText(
          'El servicio no está disponible en este momento.',
        ),
      ).toBeVisible()
      expect(within(form).getByRole('textbox', { name: 'Nombre' })).toHaveValue(
        'Instalación',
      )
      expect(
        within(form).getByRole('button', { name: 'Agregar etapa' }),
      ).toBeEnabled()
    })

    it('keeps what was typed when there is no connection', async () => {
      const { user } = await renderStages()
      server.use(http.post(`${base}/stages`, () => HttpResponse.error()))
      await createStage(user, 'Instalación')
      const form = await dialog('Nueva etapa')
      expect(await within(form).findByText('No hay conexión.')).toBeVisible()
      expect(within(form).getByRole('textbox', { name: 'Nombre' })).toHaveValue(
        'Instalación',
      )
    })

    it('shows the unknown field of a 422 as a general error and reads the list again', async () => {
      const { user, stages } = await renderStages()
      server.use(
        http.post(`${base}/stages`, () =>
          problemResponse('validation_failed', 422, {
            errors: [{ field: 'position', code: 'invalid_value' }],
          }),
        ),
      )
      await createStage(user, 'Instalación')
      const form = await dialog('Nueva etapa')
      expect(
        await within(form).findByText('No pudimos guardar el cambio.'),
      ).toBeVisible()
      expect(
        within(form).getByText('Actualizamos la lista: probá de nuevo.'),
      ).toBeVisible()
      await waitFor(() => expect(stages.reads).toBe(2))
    })

    it('applies a field error of a 422 to its field', async () => {
      const { user } = await renderStages()
      server.use(
        http.post(`${base}/stages`, () =>
          problemResponse('validation_failed', 422, {
            errors: [{ field: 'name', code: 'invalid_value' }],
          }),
        ),
      )
      await createStage(user, 'Instalación')
      expect(await screen.findByText('Revisá este dato.')).toBeVisible()
      expect(screen.getByRole('textbox', { name: 'Nombre' })).toHaveFocus()
    })

    it('sends a single request when the button is activated twice', async () => {
      const held = gate()
      const { user, stages } = await renderStages()
      server.use(
        http.post(`${base}/stages`, async ({ request }) => {
          stages.writes.push({
            call: 'POST /stages',
            body: await request.json(),
          })
          await held.opened
          return HttpResponse.json(
            { ...genericStages()[0]!, id: 'nueva', name: 'Instalación' },
            { status: 201 },
          )
        }),
      )
      await openCreateDialog(user)
      await user.type(
        screen.getByRole('textbox', { name: 'Nombre' }),
        'Instalación',
      )
      await user.dblClick(screen.getByRole('button', { name: 'Agregar etapa' }))
      expect(
        await screen.findByRole('button', { name: 'Agregando…' }),
      ).toBeDisabled()
      held.open()
      await screen.findByText('Agregaste la etapa «Instalación».')
      expect(stages.writes).toHaveLength(1)
    })

    it('drops to read-only when the server answers 403 and the session no longer has the permission', async () => {
      const { user } = await renderStages()
      let demoted = false
      server.use(
        http.get(`${base}/me`, () =>
          HttpResponse.json(demoted ? operatorSession() : adminSession()),
        ),
        http.post(`${base}/stages`, () => {
          demoted = true
          return problemResponse('forbidden', 403)
        }),
      )
      await createStage(user, 'Instalación')
      await waitFor(() =>
        expect(
          screen.queryByRole('button', { name: '+ Agregar etapa' }),
        ).toBeNull(),
      )
      expect(
        screen.getByText('Solo un administrador puede cambiar las etapas.'),
      ).toBeVisible()
      expect(screen.queryByText(/Sin permiso/)).toBeNull()
      expect(screen.queryByRole('dialog')).toBeNull()
    })
  })

  describe('editing', () => {
    it('sends only the type that changed and shows it in the row', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'Perdido', 'Editar')
      const form = await dialog('Editar etapa')
      expect(within(form).getByRole('textbox', { name: 'Nombre' })).toHaveValue(
        'Perdido',
      )
      expect(
        within(form).getByRole('radio', { name: 'Cierre perdido' }),
      ).toBeChecked()
      expect(within(form).getByRole('radio', { name: 'Gris' })).toBeChecked()
      expect(
        within(form).queryByRole('combobox', { name: 'Ubicación' }),
      ).toBeNull()
      await user.click(
        within(form).getByRole('radio', { name: 'Cierre ganado' }),
      )
      await user.click(
        within(form).getByRole('button', { name: 'Guardar etapa' }),
      )
      expect(
        await screen.findByText('Guardamos la etapa «Perdido».'),
      ).toBeVisible()
      expect(stages.writes).toEqual([
        { call: `PATCH /stages/${stageId('Perdido')}`, body: { type: 'won' } },
      ])
      expect(row('Perdido')).toHaveTextContent('Cierre ganado')
      await waitFor(async () =>
        expect(await actionsTrigger('Perdido')).toHaveFocus(),
      )
    })

    it('sends the trimmed name when only the name changed', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'Aprobado', 'Editar')
      const name = await screen.findByRole('textbox', { name: 'Nombre' })
      await user.clear(name)
      await user.type(name, '  Aceptado  ')
      await user.click(screen.getByRole('button', { name: 'Guardar etapa' }))
      await screen.findByText('Guardamos la etapa «Aceptado».')
      expect(stages.writes[0]?.body).toEqual({ name: 'Aceptado' })
    })

    it('closes without a request when nothing changed', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'Perdido', 'Editar')
      await dialog('Editar etapa')
      await user.click(screen.getByRole('button', { name: 'Guardar etapa' }))
      await waitFor(() =>
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
      )
      expect(stages.writes).toEqual([])
      await waitFor(async () =>
        expect(await actionsTrigger('Perdido')).toHaveFocus(),
      )
    })

    it('does not count the stage itself as a repeated name', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'Aprobado', 'Editar')
      const name = await screen.findByRole('textbox', { name: 'Nombre' })
      await user.clear(name)
      await user.type(name, 'APROBADO')
      await user.click(screen.getByRole('button', { name: 'Guardar etapa' }))
      await screen.findByText('Guardamos la etapa «APROBADO».')
      expect(stages.writes[0]?.body).toEqual({ name: 'APROBADO' })
    })

    it('rejects turning the only "Cierre ganado" stage into an open one', async () => {
      const { user, stages } = await renderStages()
      await runRowAction(user, 'Finalizado', 'Editar')
      await user.click(await screen.findByRole('radio', { name: 'Abierta' }))
      await user.click(screen.getByRole('button', { name: 'Guardar etapa' }))
      expect(
        await screen.findByText(
          'Tiene que quedar al menos una etapa de cierre ganado.',
        ),
      ).toBeVisible()
      expect(stages.writes).toEqual([])
    })

    it('shows the generic stage error inside the dialog when the list was old', async () => {
      const { user, stages } = await renderStages()
      server.use(
        http.patch(`${base}/stages/:stageId`, () =>
          problemResponse('invalid_state', 409),
        ),
      )
      await runRowAction(user, 'Perdido', 'Editar')
      await user.click(
        await screen.findByRole('radio', { name: 'Cierre ganado' }),
      )
      await user.click(screen.getByRole('button', { name: 'Guardar etapa' }))
      const form = await dialog('Editar etapa')
      expect(
        await within(form).findByText(
          'No pudimos hacer ese cambio en las etapas.',
        ),
      ).toBeVisible()
      await waitFor(() => expect(stages.reads).toBe(2))
    })

    it('closes the dialog and says the stage is gone when the edit answers 404', async () => {
      const { user, stages } = await renderStages()
      server.use(
        http.patch(`${base}/stages/:stageId`, () =>
          problemResponse('not_found', 404),
        ),
      )
      await runRowAction(user, 'Perdido', 'Editar')
      await user.click(
        await screen.findByRole('radio', { name: 'Cierre ganado' }),
      )
      await user.click(screen.getByRole('button', { name: 'Guardar etapa' }))
      expect(
        await screen.findByText(
          'Esa etapa ya no existe. Actualizamos la lista.',
        ),
      ).toBeVisible()
      expect(screen.queryByRole('dialog')).toBeNull()
      await waitFor(() => expect(stages.reads).toBe(2))
    })
  })

  it('opens the menu of a row with the pointer too, and "Eliminar" is never disabled on an idle row', async () => {
    const { user } = await renderStages()
    await user.click(await actionsTrigger('Aprobado'))
    expect(
      screen.getByRole('menuitem', { name: 'Eliminar' }),
    ).not.toHaveAttribute('aria-disabled')
    await pickMenuItem(user, 'Editar')
    await dialog('Editar etapa')
  })
})
