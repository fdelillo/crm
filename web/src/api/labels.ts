import {
  DEFAULT_LABELS,
  terminologyFrom,
  type Terminology,
} from '@/lib/terminology'

const defaultTerminology = terminologyFrom(DEFAULT_LABELS)

// T-002-F101 (002/DD-F2): provisional, without a query; T-002-F501 connects it to the labels read.
export function useTerminology(): Terminology {
  return defaultTerminology
}
