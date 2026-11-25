import type { GrammaticalGender, Labels } from '@/api/types'

// T-002-F101 (002/DD-F3, ADR-030): grammar of the company's own names, pure and without React.
export type TermEntity = 'project' | 'customer' | 'quote'

export interface EntityTerm {
  singular: string
  plural: string
  gender: GrammaticalGender
  lower: { singular: string; plural: string }
  article: {
    the: 'el' | 'la'
    theP: 'los' | 'las'
    a: 'un' | 'una'
    ofThe: 'del' | 'de la'
    toThe: 'al' | 'a la'
  }
}
export type Terminology = Record<TermEntity, EntityTerm>

// The names of the industry templates; the fallback when reading the labels fails (DD-F2).
export const DEFAULT_LABELS: Labels = {
  project: { singular: 'Proyecto', plural: 'Proyectos', gender: 'masculine' },
  customer: { singular: 'Cliente', plural: 'Clientes', gender: 'masculine' },
  quote: {
    singular: 'Presupuesto',
    plural: 'Presupuestos',
    gender: 'masculine',
  },
}

// Lowercases the first letter only when the rest of the first word is already lowercase,
// so acronyms and mixed case ("OT", "PyME") stay as written.
export function midSentence(text: string): string {
  const [first = ''] = text.split(' ')
  const rest = first.slice(1)
  if (first === '' || rest !== rest.toLowerCase()) return text
  return first.toLowerCase() + text.slice(first.length)
}

export function byGender<T>(
  gender: GrammaticalGender,
  forms: { masculine: T; feminine: T },
): T {
  return forms[gender]
}

function entityTerm(label: Labels[TermEntity]): EntityTerm {
  const { gender } = label
  return {
    singular: label.singular,
    plural: label.plural,
    gender,
    lower: {
      singular: midSentence(label.singular),
      plural: midSentence(label.plural),
    },
    article: {
      the: byGender(gender, { masculine: 'el', feminine: 'la' }),
      theP: byGender(gender, { masculine: 'los', feminine: 'las' }),
      a: byGender(gender, { masculine: 'un', feminine: 'una' }),
      ofThe: byGender(gender, { masculine: 'del', feminine: 'de la' }),
      toThe: byGender(gender, { masculine: 'al', feminine: 'a la' }),
    },
  }
}

export function terminologyFrom(labels: Labels): Terminology {
  return {
    project: entityTerm(labels.project),
    customer: entityTerm(labels.customer),
    quote: entityTerm(labels.quote),
  }
}
