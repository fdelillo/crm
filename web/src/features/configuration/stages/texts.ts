import { byGender, type Terminology } from '@/lib/terminology'

const capitalized = (text: string) =>
  text.charAt(0).toUpperCase() + text.slice(1)

// T-002-F202 (002/BR-F11, ui.md §9.2, §14.2): the stage texts that name the project with the
// company's own words, in one place. Everything else on the screen is fixed text.
export function stageTexts(terms: Terminology) {
  const { project } = terms
  const them = `${project.article.theP} ${project.lower.plural}`
  const closed = byGender(project.gender, {
    masculine: 'cerrados',
    feminine: 'cerradas',
  })
  const fresh = byGender(project.gender, {
    masculine: 'nuevos',
    feminine: 'nuevas',
  })
  return {
    title: `Etapas de ${them}`,
    description: `El orden en que avanzan ${them}. Las nuevas empiezan en la etapa inicial.`,
    typeDescriptions: {
      open: `${capitalized(them)} siguen en curso.`,
      won: `${capitalized(them)} en esta etapa se consideran ${closed}: se hicieron.`,
      lost: `Se consideran ${closed}: no se hicieron.`,
    },
    firstNotOpen: `La primera etapa tiene que ser abierta: es donde empiezan ${them} ${fresh}.`,
    noWon: 'Tiene que quedar al menos una etapa de cierre ganado.',
  }
}
export type StageTexts = ReturnType<typeof stageTexts>
