/// <reference types="node" />
import { Blob as NodeBlob, File as NodeFile } from 'node:buffer'
import { vi } from 'vitest'

// jsdom's File and FormData cannot cross into Node's fetch (the multipart body breaks and the
// handler cannot parse it), so the logo tests build uploads with Node's classes instead.
export { NodeBlob, NodeFile }

export async function useNodeMultipart() {
  // Node's FormData is only reachable through a Response, jsdom hides the global one.
  const sample = await new Response(new URLSearchParams('a=b')).formData()
  vi.stubGlobal('FormData', sample.constructor)
  vi.stubGlobal('File', NodeFile)
}

export function nodeFile(
  bytes: Uint8Array<ArrayBuffer>,
  name: string,
  type: string,
) {
  return new NodeFile([bytes], name, { type }) as unknown as File
}
export const nodeBlob = (size: number) =>
  new NodeBlob([new Uint8Array(size)]) as unknown as Blob
