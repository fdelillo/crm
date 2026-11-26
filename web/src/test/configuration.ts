import { http, HttpResponse } from 'msw'
import type {
  SessionInfo,
  Stage,
  StageColor,
  StageCreate,
  StageType,
  StageUpdate,
} from '@/api/types'
import { base, problemResponse } from '@/test/company'
import {
  buildCurrentUser,
  buildSessionInfo,
  buildTenant,
} from '@/test/fixtures'
import { server } from '@/test/msw/server'

// T-002-F201 (002/ui.md §16.4): fixtures of the configuration screens, with made-up data.
const stageIdPrefix = '0199bada-0000-7000-8000-0000000002'

export function buildStage(overrides: Partial<Stage> = {}): Stage {
  return {
    id: `${stageIdPrefix}00`,
    name: 'Etapa de prueba',
    type: 'open',
    color: 'blue',
    position: 1,
    ...overrides,
  }
}

function stageList(
  rows: ReadonlyArray<readonly [string, StageType, StageColor]>,
): Stage[] {
  return rows.map(([name, type, color], index) =>
    buildStage({
      id: `${stageIdPrefix}${String(index + 1).padStart(2, '0')}`,
      name,
      type,
      color,
      position: index + 1,
    }),
  )
}

/** The pipeline of the "Genérico" company: Presupuesto, Aprobado, En curso, Finalizado, Perdido. */
export function genericStages(): Stage[] {
  return stageList([
    ['Presupuesto', 'open', 'blue'],
    ['Aprobado', 'open', 'teal'],
    ['En curso', 'open', 'yellow'],
    ['Finalizado', 'won', 'green'],
    ['Perdido', 'lost', 'gray'],
  ])
}

/** The "cuatro etapas" variant, without Perdido. */
export function fourStages(): Stage[] {
  return genericStages().slice(0, 4)
}

// T-002-F202: the session of each role and an in-memory /stages server, so the screens run against
// the real hooks with the network answered at the HTTP edge (MSW).
export function adminSession() {
  return buildSessionInfo({
    user: buildCurrentUser({ role: 'admin', email_verified: true }),
    permissions: ['settings.manage'],
  })
}

export function operatorSession() {
  return buildSessionInfo({
    user: buildCurrentUser({ role: 'operator', email_verified: true }),
    permissions: [],
  })
}

/** The two requests every screen inside the shell makes: /me and /tenant. */
export function serveSession(session: SessionInfo = adminSession()) {
  server.use(
    http.get(`${base}/me`, () => HttpResponse.json(session)),
    http.get(`${base}/tenant`, () => HttpResponse.json(buildTenant())),
  )
}

export interface StagesServer {
  /** The list the server holds now (always in position order). */
  stages: Stage[]
  /** Writes in the order they ARRIVED, as "METHOD /path" plus their JSON body when they have one. */
  writes: Array<{ call: string; body: unknown }>
  /** How many `GET /stages` arrived. */
  reads: number
  /** Free-form markers the test adds, in the same order as the writes (see `mark`). */
  timeline: string[]
  mark: (event: string) => void
}

export interface StagesServerOptions {
  session?: SessionInfo
  stages?: Stage[]
  /** Holds the RESPONSE of `POST /stages/{id}/move` (the server already applied the move). */
  holdMoveResponse?: Promise<void>
  /** Holds the RESPONSE of `GET /stages` (it already took its snapshot of the list). */
  holdReadResponse?: Promise<void>
}

const renumbered = (stages: Stage[]) =>
  stages.map((stage, index) => ({ ...stage, position: index + 1 }))

/** /me, /tenant and the five /stages routes, with the rules of the real server for the cases we test. */
export function serveStages(options: StagesServerOptions = {}): StagesServer {
  let created = 0
  const server_: StagesServer = {
    stages: renumbered(options.stages ?? genericStages()),
    writes: [],
    reads: 0,
    timeline: [],
    mark(event) {
      server_.timeline.push(event)
    },
  }
  const record = (call: string, body?: unknown) => {
    server_.writes.push({ call, body })
    server_.timeline.push(call)
  }
  const notFound = () => problemResponse('not_found', 404)
  serveSession(options.session ?? adminSession())
  server.use(
    http.get(`${base}/stages`, async () => {
      server_.reads += 1
      const snapshot = server_.stages
      if (options.holdReadResponse && server_.reads > 1)
        await options.holdReadResponse
      return HttpResponse.json({ items: snapshot })
    }),
    http.post(`${base}/stages`, async ({ request }) => {
      const body = (await request.json()) as StageCreate
      record('POST /stages', body)
      const taken = server_.stages.some(
        (stage) => stage.name.toLowerCase() === body.name.toLowerCase(),
      )
      if (taken) return problemResponse('name_taken', 409)
      created += 1
      const stage: Stage = {
        id: `${stageIdPrefix}9${created}`,
        name: body.name,
        type: body.type,
        color: body.color,
        position: 0,
      }
      const index = (body.position ?? server_.stages.length + 1) - 1
      const next = [...server_.stages]
      next.splice(index, 0, stage)
      server_.stages = renumbered(next)
      return HttpResponse.json(
        server_.stages.find((item) => item.id === stage.id),
        { status: 201 },
      )
    }),
    http.patch(`${base}/stages/:stageId`, async ({ request, params }) => {
      const body = (await request.json()) as StageUpdate
      record(`PATCH /stages/${String(params.stageId)}`, body)
      const current = server_.stages.find((item) => item.id === params.stageId)
      if (!current) return notFound()
      const changed = { ...current, ...body }
      server_.stages = server_.stages.map((item) =>
        item.id === current.id ? changed : item,
      )
      return HttpResponse.json(changed)
    }),
    http.delete(`${base}/stages/:stageId`, ({ params }) => {
      record(`DELETE /stages/${String(params.stageId)}`)
      if (!server_.stages.some((item) => item.id === params.stageId))
        return notFound()
      server_.stages = renumbered(
        server_.stages.filter((item) => item.id !== params.stageId),
      )
      return new HttpResponse(null, { status: 204 })
    }),
    http.post(`${base}/stages/:stageId/move`, async ({ request, params }) => {
      const body = (await request.json()) as { position: number }
      record(`POST /stages/${String(params.stageId)}/move`, body)
      const moving = server_.stages.find((item) => item.id === params.stageId)
      if (!moving) return notFound()
      const rest = server_.stages.filter((item) => item.id !== moving.id)
      rest.splice(body.position - 1, 0, moving)
      server_.stages = renumbered(rest)
      const answer = { items: server_.stages }
      if (options.holdMoveResponse) await options.holdMoveResponse
      server_.mark('move answered')
      return HttpResponse.json(answer)
    }),
  )
  return server_
}

export const stageId = (name: string, stages: Stage[] = genericStages()) => {
  const found = stages.find((stage) => stage.name === name)
  if (!found) throw new Error(`No stage called ${name}`)
  return found.id
}
