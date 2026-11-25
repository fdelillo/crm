import { describe, expect, it } from 'vitest'
import type { Labels } from '@/api/types'
import {
  byGender,
  DEFAULT_LABELS,
  midSentence,
  terminologyFrom,
} from './terminology'

describe('midSentence', () => {
  // T-002-F101 (002/DD-F3, ADR-030): lowercase only a capitalised word, never acronyms.
  it.each([
    ['Obra', 'obra'],
    ['Orden de trabajo', 'orden de trabajo'],
    ['Ñandú', 'ñandú'],
    ['OT', 'OT'],
    ['PyME', 'PyME'],
    ['obra', 'obra'],
    ['', ''],
  ])('turns %j into %j', (text, expected) => {
    expect(midSentence(text)).toBe(expected)
  })
})

describe('byGender', () => {
  // T-002-F101 (002/§9.2): picks the agreeing form.
  it('picks the form for the gender', () => {
    const forms = { masculine: 'Nuevo', feminine: 'Nueva' }
    expect(byGender('feminine', forms)).toBe('Nueva')
    expect(byGender('masculine', forms)).toBe('Nuevo')
  })
})

describe('DEFAULT_LABELS', () => {
  // T-002-F101 (002/§9.2): the template names, all masculine (spec, "Comunes a las dos plantillas").
  it('matches the names shared by both templates', () => {
    expect(DEFAULT_LABELS).toEqual({
      project: {
        singular: 'Proyecto',
        plural: 'Proyectos',
        gender: 'masculine',
      },
      customer: {
        singular: 'Cliente',
        plural: 'Clientes',
        gender: 'masculine',
      },
      quote: {
        singular: 'Presupuesto',
        plural: 'Presupuestos',
        gender: 'masculine',
      },
    })
  })
})

describe('terminologyFrom', () => {
  // T-002-F101 (002/DD-F3, ADR-030): articles and lowercase forms follow the gender.
  it('describes the default names', () => {
    expect(terminologyFrom(DEFAULT_LABELS).project).toEqual({
      singular: 'Proyecto',
      plural: 'Proyectos',
      gender: 'masculine',
      lower: { singular: 'proyecto', plural: 'proyectos' },
      article: {
        the: 'el',
        theP: 'los',
        a: 'un',
        ofThe: 'del',
        toThe: 'al',
      },
    })
  })

  it('agrees with a feminine custom name', () => {
    const labels: Labels = {
      ...DEFAULT_LABELS,
      project: { singular: 'Obra', plural: 'Obras', gender: 'feminine' },
    }
    expect(terminologyFrom(labels).project).toEqual({
      singular: 'Obra',
      plural: 'Obras',
      gender: 'feminine',
      lower: { singular: 'obra', plural: 'obras' },
      article: {
        the: 'la',
        theP: 'las',
        a: 'una',
        ofThe: 'de la',
        toThe: 'a la',
      },
    })
  })

  it('resolves the three entities independently', () => {
    const labels: Labels = {
      ...DEFAULT_LABELS,
      customer: {
        singular: 'Comitente',
        plural: 'Comitentes',
        gender: 'masculine',
      },
      quote: {
        singular: 'Cotización',
        plural: 'Cotizaciones',
        gender: 'feminine',
      },
    }
    const t = terminologyFrom(labels)
    expect(t.customer.article.toThe).toBe('al')
    expect(t.quote.article.a).toBe('una')
    expect(t.quote.lower.plural).toBe('cotizaciones')
  })
})
