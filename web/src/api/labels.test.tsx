import { expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { DEFAULT_LABELS, terminologyFrom } from '@/lib/terminology'
import { renderApp } from '@/test/renderApp'
import { useTerminology } from './labels'

function Probe() {
  const t = useTerminology()
  return <p>{JSON.stringify(t)}</p>
}

// T-002-F101 (002/DD-F2): until T-002-F501 connects the query, the default names, with no request.
it('gives the default terminology without any request', async () => {
  const fetchSpy = vi.spyOn(globalThis, 'fetch')
  renderApp(['/probe'], { routes: [{ path: '/probe', Component: Probe }] })
  const text = (await screen.findByText(/"project"/)).textContent
  expect(JSON.parse(text ?? '')).toEqual(terminologyFrom(DEFAULT_LABELS))
  expect(fetchSpy).not.toHaveBeenCalled()
})
