# ADR-030: Terminología configurable en la interfaz (nombres personalizados con género)

**Status**: Proposed (pendiente de aprobación del usuario con `ui.md` de 002)
**Fecha**: 2026-10-23
**Origen**: spec 002, Historia 2 y P-3 (`specs/002-configuracion/ui.md` §9, DD-F2 y DD-F3 de 002);
rige para toda pantalla con sesión de las specs 003 a 010

## Contexto

La spec 002 (FR-002, P-3) permite que cada empresa llame a Proyecto, Cliente y Presupuesto con otro
nombre ("Obra", "Comitente", "Cotización"), con singular, plural y **género gramatical**, y pide que
**toda la interfaz** use esos nombres con la concordancia correcta ("Nueva obra", "Las obras
abiertas"). Los PDF los usan desde 005 (lado servidor) y los emails no los usan.

El contrato de 002 expone `GET /settings/labels` (cualquier rol) con `{ project, customer, quote }`,
cada uno `{ singular, plural, gender: masculine | feminine }`. ADR-023 descartó una librería de i18n:
los textos se escriben en español directamente en los componentes.

Fuerzas en tensión:

- Las specs 003 a 010 van a escribir cientos de textos con estos sustantivos (navegación, títulos,
  botones, estados vacíos, confirmaciones). Si cada pantalla resuelve la concordancia a su manera,
  los errores ("Nuevo obra") aparecen pantalla por pantalla.
- Los nombres llegan del servidor (server state): no pueden copiarse a un Context ni a un store
  (INV-F01 de 001).
- El sustantivo lo escribe el Administrador con mayúscula inicial ("Obra"), pero en medio de una
  frase va en minúscula ("Nueva obra"), salvo siglas ("OT").
- Un texto con el nombre equivocado durante la carga ("Proyectos" y medio segundo después "Obras")
  es peor que esperar un instante.

## Decisión

1. **Fuente**: los nombres viven en la caché de TanStack Query, clave `['settings', 'labels']`
   (`api/labels.ts`). Se piden apenas hay sesión (precarga desde el guard de sesión, en paralelo con
   el chunk del shell) y el contenido del shell (`<main>`) espera a que esa lectura termine, con
   éxito o con error. Si falla, se usan los nombres por defecto (los de las plantillas: Proyecto,
   Cliente y Presupuesto, masculinos) y la query se vuelve a intentar con las reglas generales.
2. **Forma de uso**: una función pura `terminologyFrom(labels)` (`lib/terminology.ts`) arma, para
   cada entidad, el singular y el plural tal como se escribieron, sus formas para el medio de una
   frase y los artículos y contracciones concordantes (`el`/`la`, `los`/`las`, `un`/`una`,
   `del`/`de la`, `al`/`a la`). Los componentes la leen con el hook `useTerminology()` y arman sus
   frases con `byGender(gender, { masculine, feminine })` para adjetivos y participios.
3. **Minúscula en medio de la frase**: se pasa a minúscula solo la primera letra, y solo si el resto
   de la primera palabra ya está en minúscula ("Obra" → "obra", "Orden de trabajo" → "orden de
   trabajo"; "OT" y "PyME" quedan iguales).
4. **Alcance**: todo texto de una pantalla con sesión que nombre a esas entidades. Las pantallas
   públicas (registro, ingreso, enlaces de email) no tienen sesión y no los usan. Los **datos**
   (nombres de etapas y de categorías, aunque digan "Presupuesto" o "Cobros de clientes") no se
   reescriben: los edita el Administrador.
5. **Vigilancia**: un test recorre `src/**/*.tsx` (fuera de tests y de las pantallas públicas) y
   falla si encuentra los literales "proyecto(s)", "cliente(s)" o "presupuesto(s)" en un texto
   visible.

## Fundamento

- Un solo lugar resuelve concordancia y mayúsculas: las specs siguientes escriben
  `` `${byGender(t.project.gender, { masculine: 'Nuevo', feminine: 'Nueva' })} ${t.project.lower.singular}` ``
  y no repiten reglas.
- El hook lee la caché: no hay copia de datos del servidor (INV-F01 de 001) y un cambio de nombres
  (`PUT /settings/labels`) se ve en toda la interfaz sin recargar.
- Esperar la lectura antes de pintar el contenido evita el parpadeo del nombre equivocado; la
  precarga en paralelo con el chunk del shell lo deja en casi cero en una carga normal.
- Los nombres por defecto ante un error mantienen la app usable: el peor caso es leer "Proyecto"
  donde la empresa dice "Obra", nunca una pantalla bloqueada.

## Alternativas consideradas

- **Librería de i18n con `select` por género (FormatJS/ICU, i18next)**: resuelve concordancias con
  mensajes declarativos, pero ADR-023 descartó i18n para un solo idioma; sumaría catálogos de
  mensajes y una API más para dos valores de género.
- **Frases ya armadas por el servidor** (p. ej. `new_project_label: "Nueva obra"`): el backend no
  conoce la UI; cada texto nuevo sería un cambio de contrato.
- **Copiar los nombres a un Context propio al arrancar**: duplica server state (INV-F01 de 001) y
  obliga a sincronizarlo tras el `PUT`.
- **No esperar la lectura y usar los nombres por defecto mientras carga**: sin espera, pero muestra
  "Proyectos" y después "Obras" en cada carga de una empresa que los cambió.
- **Bloquear con un error si la lectura falla**: un dato cosmético dejaría la app inutilizable.
- **Guardar también la forma en minúscula** (pedirle al Administrador un campo más): exacto, pero
  agrega un concepto por entidad (principio I); la regla de la primera letra cubre los casos
  previsibles, incluidas las siglas.

## Consecuencias

- Ganás: concordancia y mayúsculas correctas en un solo lugar; cambiar los nombres actualiza la
  interfaz al instante; las specs siguientes tienen una API chica y probada.
- Aceptás: una lectura más al arrancar la sesión (en paralelo con el chunk del shell; T-002-F604
  mide el LCP de `/`); una regla heurística de minúsculas que se equivoca con un nombre propio de una
  sola palabra ("Fiat" se mostraría "fiat" en medio de una frase); y los casos que dos géneros no
  cubren (sustantivos femeninos con "a" tónica: "el área") se redactan sin artículo. Los datos
  sembrados que repiten esas palabras ("Cobros de clientes", la etapa "Presupuesto") no cambian
  solos.
