// T-002-F101 (002/INV-F10, NFR-F5): the server counts code points, not UTF-16 units.
export function countCodePoints(text: string): number {
  return Array.from(text).length
}
