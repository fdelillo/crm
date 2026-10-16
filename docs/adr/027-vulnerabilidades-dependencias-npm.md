# ADR-027: Vulnerabilidades conocidas en dependencias npm del frontend con `npm audit`

**Status**: Accepted (aprobado por el usuario el 2026-10-16: `ui.md` P-F13)
**Fecha**: 2026-10-15
**Origen**: fila de coordinación T-F009 ↔ ADR-026 de `specs/001-empresas-usuarios/tasks.md`
(Backend, "Coordinación con la sección Frontend"), que deja la decisión al `frontend-architect`; cierre
de la Fase F0 (`npm audit` con 7 altas). Complementa ADR-026, que deja npm fuera de su alcance. Aplica
a todas las specs.

## Contexto

ADR-026 cubre las dependencias Go con govulncheck y deja afuera las de `web/`. La Fase F0 cerró sin
escaneo de npm. Estado al 2026-10-15, sobre el `package-lock.json` de `main` (`de4d951`):

- `npm audit --omit=dev`: 0 vulnerabilidades.
- `npm audit`: 7 altas. Todas vienen del mismo aviso, GHSA-vfj7-8cjw-p6xm (CVE-2026-93687), por la
  cadena `shadcn` → `fast-glob` → `micromatch` → `braces`. El aviso afecta a `braces` ≤ 3.0.3: la
  recursión no tiene límite de profundidad, así que un patrón de llaves muy anidado desborda la pila y
  tira el proceso de Node. El impacto es solo de disponibilidad, y el aviso **no tiene versión
  corregida** a esa fecha.
- `shadcn` está en `devDependencies`, pero el build lo necesita: `src/styles/globals.css` importa
  `shadcn/tailwind.css`. El build usa una hoja de estilos del paquete; el código vulnerable es el del
  CLI (`npx shadcn add …`), que corre solo cuando un desarrollador agrega componentes.

Hay tres diferencias con govulncheck que cambian el diseño:

1. **npm no analiza alcance.** Informa un paquete vulnerable aunque ninguna línea lo llame. La
   severidad sale del aviso (CVSS), no del uso. Si se bloquea por todo, el job falla por ruido y el
   equipo aprende a ignorarlo, que es lo que ADR-026 quiso evitar con el nivel por símbolo.
2. **`npm audit` no permite excluir un aviso por id.** No tiene lista de excepciones ni vencimientos.
3. **Lo que el usuario ejecuta y lo que el equipo ejecuta son árboles distintos.** El navegador recibe
   solo lo que el bundle incluye. El resto corre en el equipo del desarrollador y en CI.

Fuerzas: frenar en el PR lo que llega al navegador; ver lo que se publica entre PRs; no bloquear PRs
por avisos de herramientas de desarrollo que no tienen corrección ni impacto real; no sumar
dependencias ni servicios; reusar la política, el workflow y el registro de excepciones de ADR-026.

## Decisión

1. **Herramienta.** `npm audit`, del npm que trae el Node fijado en `web/.nvmrc` (hoy 24.22.0). No
   suma dependencias. La base de avisos es la del registro npm en el momento de la corrida (GitHub
   Advisory Database) y no se fija.

2. **Clasificación de dependencias.** Define qué se escanea y con qué umbral, así que pasa a ser una
   regla del proyecto:
   - **`dependencies`**: todo paquete cuyo código JavaScript llega a `web/dist`, es decir, lo que corre
     en el navegador del usuario.
   - **`devDependencies`**: lo que corre solo en equipos de desarrollo y en CI (build, tests, lint,
     generadores de tipos y el CLI de shadcn), más los paquetes cuyo único aporte al build es CSS:
     `tailwindcss`, `tw-animate-css` y `shadcn` (por `shadcn/tailwind.css`). Una hoja de estilos fija
     no ejecuta código. Los avisos de npm describen código JavaScript del paquete o de sus
     dependencias, y ese código no llega a `dist`.
   - Un archivo de `src/` que no es de test puede importar una `devDependency` solo si queda fuera del
     bundle de producción. Hoy el único caso es `@tanstack/react-query-devtools`, detrás de
     `import.meta.env.DEV` en `main.tsx`. Los archivos de test (`src/test/**`, `*.test.*`,
     `*.test-d.ts`) no cuentan.
   - Se controla en revisión (I-2). La alternativa automática está en "Alternativas".

3. **Alcance y umbral.** `make web-vuln` corre dos pasadas en `web/`:

   | Pasada | Comando | Bloquea | Lo demás |
   |---|---|---|---|
   | Producción | `npm audit --omit=dev --audit-level=low` | Cualquier aviso de severidad `low` o mayor | — |
   | Árbol completo (producción y desarrollo) | `npm audit --audit-level=critical` | Solo `critical` | Se lista en el log del job (las 7 altas de hoy aparecen ahí) |

   Las dos pasadas corren siempre, aunque falle la primera, así el log muestra el árbol completo. El
   target falla si falla cualquiera. Producción bloquea desde `low` porque npm no analiza alcance y el
   conjunto es chico (14 paquetes directos): un aviso ahí es código que corre en el navegador, y un
   falso positivo se resuelve con la tabla de §6. Desarrollo bloquea solo `critical`: los avisos
   `high` de herramientas suelen necesitar una entrada del atacante (un patrón, un archivo de
   configuración) que acá escribe el propio equipo, como pasa con `braces`.

4. **Política de falla.** Es la de ADR-026 §3: bloquea en cada PR y en cada push a `main`, y corre una
   vez por día sobre `main`, en el mismo workflow `vulncheck`. El aviso de la corrida diaria es el
   email de la corrida fallida, igual que en ADR-026. `make web-vuln` es un target propio y **no**
   forma parte de `make web-check` ni de `make check-all`: necesita red y su resultado cambia con la
   fecha.

5. **Workflow.** Se suma un segundo job a `.github/workflows/vulncheck.yml`, con los mismos
   disparadores (`pull_request`, `push` a `main`, `schedule` `27 10 * * *`, `workflow_dispatch`) y
   `permissions: contents: read`:
   - id `npm-audit`, nombre visible **`make web-vuln`**, `ubuntu-latest`, `timeout-minutes: 10`;
   - `actions/checkout` con el mismo SHA que `ci.yml` y `persist-credentials: false`;
   - `actions/setup-node` con el mismo SHA que `ci.yml`, `node-version-file: web/.nvmrc` y **sin
     caché**. No hay instalación que cachear;
   - **sin `npm ci`**: `npm audit` lee `package-lock.json`, así que en este job no corre ningún script
     de instalación de terceros (supuesto S-F14 de `ui.md`; si no se cumple, `npm ci --ignore-scripts`);
   - un solo paso, `make web-vuln`, sin `continue-on-error` y sin `|| true`.

   El check **`make web-vuln`** pasa a ser obligatorio en el ruleset de `main` cuando el workflow con
   el job nuevo está en `main`. Antes no, por lo mismo que la nota 2026-10-14 de ADR-026 (3): un PR
   sin el job nunca lo reportaría.

6. **Qué se hace con un hallazgo que bloquea.**

   | Hallazgo | Qué se hace | Dónde |
   |---|---|---|
   | Con versión corregida, en una dependencia directa | Subirla a la versión corregida; `npm ci`, `npm run check` y `make web-vuln` en verde | Rama propia desde `main` (`fix/vuln-<GHSA-id>`), mergeada antes que los demás PRs. Si lo introdujo el PR mismo, se arregla en ese PR |
   | Con versión corregida, en una dependencia transitiva | Primero, subir la dependencia directa que la trae. Si todavía no la acepta, `overrides` en `package.json` con la versión corregida, sin `--force` y con `npm ls --all` sin conflictos. El *override* se anota en la tabla de versiones de `tasks.md` (Frontend, convenciones), con el motivo y la condición para sacarlo | Igual que la fila anterior |
   | Sin versión corregida | En este orden: (a) dejar de usar la parte afectada o reemplazar el paquete; (b) aceptar el riesgo, que solo puede decidir el usuario (§7) | — |
   | Aviso de *malware* (cualquier severidad que bloquee) | Sacar el paquete o volver a la versión anterior en el mismo PR. Además, revisar si esa versión llegó a instalarse en algún equipo o en CI (`package-lock.json` de las ramas abiertas) y tratarlo como incidente | Rama propia, antes que cualquier otro PR |
   | Debajo del umbral (desarrollo por debajo de `critical`) | Nada obligatorio; se resuelve con la próxima actualización de la dependencia. Cada actualización de dependencias mira esa parte del log | El log del job |
   | Error de la herramienta (registro caído, respuesta inválida) | Volver a correr el job; nunca se marca como opcional | — |

7. **Excepciones.** Van en el mismo registro que fija ADR-026: `docs/seguridad/vulnerabilidades-aceptadas.md`,
   que se crea con el primer caso, Go o npm. Para npm, las columnas son: ecosistema (`npm`), id
   `GHSA-…` y alias `CVE-…`, paquete y versión, ruta de dependencia (salida de `npm explain
   <paquete>`), alcance (producción o desarrollo), por qué se acepta, quién lo aprobó, fecha y fecha
   de revisión (como máximo 30 días). Como `npm audit` no excluye por id, el mismo PR que agrega la
   primera excepción cambia `make web-vuln`: un script sin dependencias en `web/scripts/` (como
   `check-generated.mjs`) corre `npm audit --json`, descuenta solo los ids que tienen una excepción
   vigente en una lista versionada (id y vencimiento) y aplica los umbrales del punto 3. Una
   excepción vencida vuelve a fallar. El script se diseña con ese primer caso; hasta entonces,
   `make web-vuln` son los dos comandos del punto 3.

8. **Operación.** Una fila en la tabla de comandos de `tasks.md` (Frontend) y en la del README, una
   viñeta en `ui.md` §21 y una fila en la matriz de mantenimiento (`ui.md` §29).

**Estado conocido al adoptarlo.** Las 7 altas de la cadena de `shadcn` quedan informadas en el log
y no bloquean. No necesitan excepción. Se revisan cuando `shadcn` cambie `fast-glob` o cuando
`braces` publique una corrección.

## Invariantes

- **I-1 Falla cerrada.** `make web-vuln` termina con un código distinto de 0 si hay un aviso en o
  sobre el umbral de cualquiera de las dos pasadas, y también si `npm audit` no puede consultar el
  registro. El job no lleva `continue-on-error` ni `|| true`. El código de salida ante un error del
  registro es un supuesto (S-F14) que T-F010 verifica con un control.
- **I-2 Clasificación.** Ningún paquete cuyo JavaScript llega a `web/dist` está en
  `devDependencies` (punto 2). Se rompe sin que falle nada si alguien importa desde `src/` un paquete
  de `devDependencies` que termina en el bundle: la pasada de producción no lo ve. Se controla en
  revisión.
- **I-3 Permisos mínimos y sin instalación.** El job tiene solo `contents: read`, no usa secretos y no
  instala paquetes, así que no ejecuta scripts de terceros.
- **I-4 Sin dependencias vulnerables en el repo.** Los controles que prueban que el escaneo detecta
  algo usan un proyecto temporal fuera del árbol (como I-4 de ADR-026). Nunca se versiona un
  `package-lock.json` de prueba: GitHub y otros escáneres lo reportarían.

## Fundamento

- Separar producción de desarrollo es lo más parecido al análisis de alcance que npm permite. La
  pregunta que importa para el usuario es "¿corre en su navegador?", y la clasificación del punto 2
  la responde con un dato que el proyecto ya mantiene.
- Bloquear producción desde `low` mantiene la regla de ADR-026, "todo hallazgo que llega al producto
  bloquea", sin análisis por símbolo: con 14 dependencias directas, el ruido esperado es bajo.
- Bloquear desarrollo en `critical` sigue frenando lo grave del árbol de herramientas sin trabar los
  PRs por avisos como el de `braces`: sin corrección y con una entrada que controla el equipo.
- Mismo workflow y mismo registro de excepciones que ADR-026: un solo `cron`, un solo email y un solo
  lugar donde mirar qué riesgo se aceptó.
- `npm audit` ya viene con Node: no hay versión de herramienta que fijar ni dependencia que auditar.

## Alternativas consideradas

- **Solo producción (`--omit=dev`), sin mirar desarrollo.** Hoy da verde. Se descarta porque un aviso
  `critical` en una herramienta que corre en CI o en el equipo del desarrollador (que guarda la clave
  de la CA de mkcert, RF-12 de `ui.md`) pasaría sin que nadie lo vea.
- **Árbol completo con umbral `high`.** Hoy falla por las 7 altas, que no tienen corrección. Obliga a
  una excepción que se renueva cada 30 días mientras `braces` no publique arreglo, o enseña a ignorar
  el job.
- **Árbol completo sin umbral.** Lo mismo, con más ruido.
- **Solo informar (no bloquear).** Deja mergear un PR que suma una dependencia de producción
  vulnerable, que es lo principal que el control tiene que frenar (misma razón que la política (b) de
  ADR-026).
- **Pasar `shadcn` a `dependencies`.** La pasada de producción fallaría por un CLI que nunca llega al
  navegador.
- **Copiar `shadcn/tailwind.css` al repo y sacar el paquete `shadcn`.** Desaparecen los 7 avisos, pero
  la hoja hay que actualizarla a mano y el CLI pasaría a correr con `npx` sin versión en el
  `package-lock.json`, que es peor para la cadena de suministro. Se puede reconsiderar si el CLI sigue
  arrastrando avisos.
- **OSV-Scanner fijado en `tools.mod`.** Su configuración admite excepciones con vencimiento
  (`ignoreUntil`) e ignorar el grupo `dev`. Se descarta por ahora: suma un árbol grande de módulos a
  `tools.mod`, que comparte la lista con golangci-lint (consecuencia ya aceptada en ADR-026 para
  govulncheck), y no verifiqué que tenga umbral por severidad. Es la salida natural si las
  excepciones de npm se vuelven frecuentes.
- **`audit-ci` o `better-npm-audit`.** Suman una dependencia de desarrollo para filtrar la salida de
  `npm audit`. Hoy no hace falta ninguna excepción; si hace falta, un script propio sin dependencias
  alcanza (punto 7).
- **Workflow propio (`web-vulncheck`).** Duplica disparadores, `cron` y avisos. Un job más en
  `vulncheck` mantiene una sola corrida diaria.
- **Meterlo en `make web-check`.** Necesitaría red y su veredicto cambiaría con la fecha (misma razón
  que ADR-026 para `make check`).
- **Regla de ESLint que prohíba importar `devDependencies` desde `src/`** (`no-extraneous-dependencies`
  de `eslint-plugin-import-x`). Automatizaría I-2, pero suma un plugin y necesita excepciones
  (devtools, `src/test/**`). Se reconsidera si I-2 se rompe una vez.
- **Alertas de Dependabot.** No bloquean un PR. No se descartan como complemento.
- **`npm audit signatures`.** Verifica firmas y procedencia, no vulnerabilidades, y necesita
  `node_modules` instalado. Es otro control, fuera de esta decisión.

## Consecuencias

- **Se gana:**
  - ningún PR mergea una dependencia de producción con un aviso conocido;
  - lo `critical` del árbol de herramientas también frena;
  - lo que se publica aparece en un día como máximo;
  - se reproduce local con `make web-vuln`.
- **Se acepta a cambio:**
  - Sin análisis de alcance, un aviso de producción que no aplica bloquea igual. Se resuelve con
    `overrides` o con una excepción aprobada.
  - Los avisos `high` y menores de desarrollo no bloquean. Un `high` real en una herramienta de CI
    puede quedar solo en el log hasta la próxima actualización.
  - La clasificación del punto 2 depende de la revisión (I-2).
  - Un aviso de *malware* publicado con severidad menor que `critical` en un paquete de desarrollo no
    bloquearía. No verifiqué con qué severidad llegan esos avisos a `npm audit`. `npm audit` no es la
    defensa principal contra *malware*: lo son el `package-lock.json` con versiones exactas y `npm ci`.
  - Si el registro npm no responde, el job falla (falla cerrada) y se vuelve a correr.
  - El job envía el árbol de dependencias (nombres y versiones, sin código) al registro npm, igual que
    cualquier `npm install`.
