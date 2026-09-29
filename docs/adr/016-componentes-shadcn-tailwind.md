# ADR-016: Componentes con shadcn/ui (sobre Radix) y estilos con Tailwind CSS v4

**Status**: Accepted (shadcn/ui + Tailwind: decisión del usuario) · detalles (Radix como base, lucide, tokens, solo tema claro): Proposed, salvo "solo tema claro", confirmado por el usuario el 2026-09-29 (ver nota)
**Fecha**: 2026-09-29
**Origen**: spec 001 (`ui.md` §15, §18, §19)

> **Nota 2026-09-29**: el usuario confirmó que el MVP no tiene modo oscuro (P-F4 de `ui.md`). El
> resto de los detalles sigue pendiente de aprobación. Aclaración de piso de navegadores: Tailwind
> v4 exige Chrome ≥ 111, pero `ui.md` (NFR-F02) fija Chrome/Edge ≥ 112 por la preparación del logo
> (`createImageBitmap` con orientación EXIF); no cambia nada de esta decisión.

## Contexto

La interfaz tiene diálogos de confirmación, menús de acciones, grupos de opciones, selects,
avisos y toasts, todo accesible (WCAG 2.2 AA) y usable en el celular. Escribir a mano un diálogo o
un menú accesibles (foco atrapado, Esc, flechas, devolución del foco, ARIA) es mucho más caro de lo
que parece. Hay que elegir una sola estrategia de estilos: mezclar varias es lo que no funciona.

## Decisión

- **Tailwind CSS v4** como única estrategia de estilos (clases utilitarias + variables CSS). No se
  usan CSS Modules ni CSS-in-JS.
- **shadcn/ui**: los componentes se copian al proyecto con su CLI en `src/components/ui/` y pasan
  a ser código propio. Base **Radix UI** (la más documentada de las dos que ofrece shadcn), estilo
  por defecto, íconos **lucide-react** (los del set de shadcn).
- Formularios con los componentes `Field` de shadcn (etiqueta, descripción y error enlazados) sobre
  React Hook Form (ADR-021).
- Toasts con `sonner` (el componente de shadcn).
- **Tokens**: las variables CSS de shadcn (`--background`, `--primary`, `--destructive`,
  `--input`, `--ring`…) más `--success` y `--warning` propios, con los valores de `ui.md` §19. Los
  componentes usan tokens, nunca colores sueltos.
- **Solo tema claro** en el MVP; la estructura de variables deja el oscuro para una spec futura.
- Los componentes de `components/ui/` se ajustan solo en tokens y variantes; la lógica de la app
  va en componentes propios que los usan.

## Fundamento

- Radix resuelve la accesibilidad de los patrones difíciles (diálogo, menú, radio, select) y
  shadcn le pone un estilo razonable encima: se obtiene accesibilidad sin escribirla a mano.
- Al copiarse al proyecto, los componentes se leen y se modifican como cualquier archivo propio;
  no hay una API de librería con estilos que "pelear".
- Tailwind v4 se configura en CSS (sin archivo de configuración JS) y las clases viven junto al
  marcado: para alguien que aprende, el estilo de un componente está en un solo lugar.
- lucide es el set que usan los ejemplos de shadcn: consistencia sin decisiones extra.

## Alternativas consideradas

- **MUI / Mantine / Chakra** (librerías con estilos incluidos): muy completas, pero con su propio
  sistema de temas y de estilos, bundles más grandes y un aspecto difícil de alejar del default.
- **Base UI como base de shadcn**: alternativa válida y más nueva; menos ejemplos y respuestas
  disponibles hoy que Radix.
- **React Aria (Adobe)**: excelente accesibilidad, pero más bajo nivel (hay que estilizar todo) y
  menos material en español/ejemplos con Tailwind.
- **Componentes a mano con CSS Modules**: control total, pero la accesibilidad de diálogos, menús
  y selects se reescribe peor.
- **Tailwind v3.4**: soporta navegadores más viejos, pero shadcn actual y su documentación están
  en v4; los celulares objetivo cumplen los mínimos de v4 (supuesto S-F4 de `ui.md`).
- **Phosphor Icons** (lo que sugiere el catálogo de diseño del proyecto): igual de bueno, pero
  mezclar con lucide (el de shadcn) rompe la consistencia visual.

## Consecuencias

- Ganás: componentes accesibles listos, un solo sistema de estilos, código de UI legible y propio.
- Aceptás: **Tailwind v4 exige Safari ≥ 16.4, Chrome ≥ 111 y Firefox ≥ 128** (usa `@property` y
  `color-mix()`): navegadores más viejos no ven estilos. Los componentes copiados no se actualizan
  solos (actualizar es volver a correr el CLI y revisar diferencias). Algunas piezas de Radix y
  sonner insertan `<style>` en tiempo de ejecución, por eso la CSP permite `style-src
  'unsafe-inline'` (ADR-019).
