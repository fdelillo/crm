# ADR-026: Vulnerabilidades conocidas en dependencias Go con govulncheck

**Status**: Accepted (aprobada por el usuario el 2026-10-14)
**Fecha**: 2026-10-14
**Origen**: auditoría de seguridad de `b945373`, unidad bloqueada "CVE de dependencias"
(`specs/001-empresas-usuarios/revision-16-auditoria.md` §7); aplica a todas las specs

## Contexto

La auditoría de `b945373` no pudo evaluar el estado de CVE de `go.sum` ni de `tools.sum` porque
no tenía una base de advisories sin red, y recomendó correr `govulncheck` en CI. Hoy nada avisa si
una dependencia del binario, o la librería estándar de la versión de Go fijada, tiene una
vulnerabilidad publicada. La base de Go (vuln.go.dev) cambia todos los días: el veredicto depende
de la fecha, no solo del código.

Fuerzas: frenar antes del merge lo que un PR introduce; ver lo que se publica entre PRs; no
convertir `make check`, el checkpoint de cada fase, en algo que necesita red y cambia con la fecha;
no sumar servicios externos ni salir de la convención de `tools.mod`.

## Decisión

1. **Herramienta y versión.** `govulncheck` de `golang.org/x/vuln` **v1.9.0** (2026-10-11),
   fijado en `tools.mod` como directiva `tool`, junto a `sqlc` y `golangci-lint`, y corrido con
   `go tool -modfile=tools.mod govulncheck` desde el target `make vuln`. Se actualiza como las
   demás herramientas: en un PR, con `make check` y `make vuln` en verde. La base no se fija: es
   siempre la de vuln.go.dev del momento.
2. **Alcance.** Modo *source* sobre `./...` del módulo `github.com/fdelillo/crm`, nivel *symbol*
   (el default: solo falla lo que el código puede llegar a llamar, incluida la librería estándar),
   **sin** `-test` y **sin** `-tags`. Sin `-test`, los tags `integration` y `bench` no cambian nada,
   porque ningún archivo que no sea de test tiene una restricción de build. `./...` incluye los
   paquetes no-test de `internal/testsupport`; se aceptan porque compilan con `./...` y corren en
   CI. **Fuera del alcance**:
   - el código de test;
   - las herramientas de `tools.mod`: no van en el binario, solo procesan archivos del repo y
     corren en un CI que ya ejecuta código del PR;
   - las dependencias npm del frontend (las define T-F009).

   El estado de `tools.sum` (en modo binario) y el nivel *module* de `go.sum` se informan **una
   vez**, en T-B1101, para cerrar la unidad de la auditoría.
3. **Política de falla.** El escaneo bloquea en cada PR y en cada push a `main`, y además corre
   una vez por día sobre `main`. Todo vive en el workflow `vulncheck`. El aviso de la corrida
   diaria es su propia falla: GitHub manda el email de la corrida programada al usuario que creó
   el workflow o editó por última vez el `cron`, y ese usuario lo configura para recibir solo las
   fallas. `make vuln` es un target propio y **no** forma parte de `make check`.
4. **Qué se hace con un hallazgo.** `make vuln` termina en `Error 3` si hay un hallazgo y en
   `Error 1` si falló la herramienta.

   | Hallazgo | Qué se hace | Dónde |
   |---|---|---|
   | Alcanzable en la librería estándar | Subir `toolchain` en `go.mod` **y** en `tools.mod` al *patch* que lo corrige (I-2). Un *patch* no cambia el lenguaje | Rama propia desde `main` (`fix/vuln-<GO-ID>`), mergeada antes que los demás PRs. Si el hallazgo lo introdujo el PR mismo, se arregla en ese PR |
   | Alcanzable en un módulo que tiene versión corregida | Subir el módulo a la versión corregida y `go mod tidy`; `make check` y `make vuln` en verde | Igual que la fila anterior |
   | Alcanzable sin versión corregida | En este orden: (a) dejar de llamar al símbolo (el análisis es por símbolo, así que un cambio de código lo resuelve); (b) reemplazar la dependencia; (c) aceptar el riesgo, que solo puede decidir el usuario | `docs/seguridad/vulnerabilidades-aceptadas.md`, que se crea con el primer caso. Columnas: id `GO-…` y alias, módulo y versión, símbolo y traza, por qué se acepta, quién lo aprobó, fecha y fecha de revisión (como máximo 30 días). El mismo PR agrega a `make vuln` una exclusión por id con vencimiento. govulncheck no la trae, así que se diseña con ese primer caso. Una excepción vencida vuelve a fallar |
   | Solo importado o requerido, sin llamada alcanzable | Nada obligatorio; se sube con la próxima actualización de la dependencia | El log del job |
   | Error de la herramienta (red, vuln.go.dev, carga de paquetes) | Volver a correr el job; nunca se marca como opcional | — |

5. **Workflow** `.github/workflows/vulncheck.yml` (`name: vulncheck`; un job `govulncheck` con
   nombre visible `make vuln`; `ubuntu-latest`; `timeout-minutes: 10`; sin Docker):
   - Disparadores: `pull_request`, `push` a `main`, `schedule` diario a las `27 10 * * *` UTC
     (07:27 en Argentina; fuera de la hora en punto, cuando GitHub demora más los `cron`) y
     `workflow_dispatch` (para volver a verificar después de un arreglo).
   - `permissions: contents: read` a nivel de workflow; sin secretos.
   - `actions/checkout` y `actions/setup-go` fijadas por SHA, **los mismos** que usa `ci.yml`,
     con la versión en un comentario. `checkout` lleva `persist-credentials: false`.
   - `setup-go` con `go-version-file: go.mod` y `cache: false`. Con las mismas rutas que `ci.yml`
     (`go.sum`, `tools.sum`), la clave de caché sería la misma, y este job, que es corto, podría
     guardarla primero sin el build de los tests ni de golangci-lint. Supuesto a validar: la clave
     de setup-go sale del SO, de la versión de Go y del hash de esas rutas.
   - Un solo paso: `make vuln`, sin `continue-on-error` y sin `|| true`.
6. **Operación.** Una fila del runbook (plan 001 §12.3) y una fila en la matriz de mantenimiento
   (plan 001 §16).

## Invariantes

- **I-1 Falla cerrada.** `make vuln` termina con un código distinto de 0 si hay un hallazgo
  alcanzable (govulncheck devuelve 3) y si hay cualquier error (1). Usa el formato texto, porque
  con `-format json`, `sarif` u `openvex` govulncheck termina en 0 aunque haya hallazgos. El job no
  lleva `continue-on-error` ni `|| true`.
- **I-2 Una sola versión de Go.** Las líneas `toolchain` de `go.mod` y de `tools.mod` son iguales.
  Así govulncheck se compila con la misma versión que analiza (si no coinciden, la carga de
  paquetes puede fallar), y la librería estándar que informa es la que lleva el binario. El binario
  de producción se compila con esa versión; es un supuesto hasta que exista el pipeline de
  despliegue (P-1).
- **I-3 Permisos mínimos.** El workflow tiene solo `contents: read` y no usa secretos.
- **I-4 Sin dependencias vulnerables en el repo.** Los controles que prueban que el escaneo
  detecta algo usan módulos temporales fuera del árbol. Nunca se versiona un `go.mod` de prueba:
  GitHub y otros escáneres lo reportarían.

## Fundamento

- `tools.mod` es la forma en que el repo fija y corre herramientas (README, Makefile): una sola
  forma de hacerlo, y las sumas quedan en el repo.
- El nivel por símbolo reduce los falsos positivos. Eso es lo que permite que el job sea
  bloqueante sin que el equipo aprenda a ignorarlo.
- Bloquear en el PR frena lo que el cambio introduce. La corrida diaria ve lo que se publica entre
  PRs, sin depender de que haya actividad.
- `make check` sigue siendo determinista y puede correr sin acceso a la base de vulnerabilidades,
  como necesitan los agentes y las validaciones locales.

## Alternativas consideradas

- **`go run golang.org/x/vuln/cmd/govulncheck@v1.9.0`.** Aísla su lista de módulos (no sube el
  `x/tools` de golangci-lint) y sum.golang.org verifica su checksum. Se descarta porque sería una
  segunda forma de correr herramientas, con la versión en el Makefile y las sumas fuera del repo.
  Queda como salida si el `x/tools` que trae govulncheck rompe golangci-lint.
- **`golang/govulncheck-action`.** Instala `govulncheck@latest` con Go `stable` y `check-latest:
  true`: no fija ni la herramienta ni Go, y no se reproduce con `make`.
- **`go install …@latest` en el runner.** Tiene el mismo problema: la versión no queda fijada.
- **`-test -tags=integration,bench`.** Suma código que no va en el binario. Un hallazgo ahí bloquea
  PRs sin proteger producción.
- **`-mode binary` sobre `crm` compilado.** Es menos preciso (mira qué símbolos están en el
  binario, no da trazas y tiene falsos positivos) y obliga a compilar antes. El modo source ya
  cubre ese código. Se usa una sola vez, para las herramientas.
- **`-scan module`.** Falla por cualquier módulo vulnerable de `go.sum` aunque el código no lo
  llame. Es ruido que enseña a ignorar el job. Se usa una sola vez, para el informe.
- **`./cmd/...` en lugar de `./...`.** Sería más preciso (solo cuenta lo alcanzable desde `main`).
  Se descarta por ahora porque `./...` es lo estándar y es más conservador. Si `internal/testsupport`
  genera ruido seguido, se acota con una nota fechada.
- **Política (a): bloquear solo en PRs.** Lo que se publica entre PRs no aparece hasta el próximo
  PR, y sin actividad nadie se entera.
- **Política (b): no bloquear en PRs y correr una vez por día.** Deja mergear un PR que introduce
  una dependencia vulnerable, que es justamente lo principal que el job tiene que frenar.
- **Abrir un issue automático desde la corrida programada.** Necesita `issues: write` y un script:
  más permisos y más código para un repo de una persona. El email de la corrida fallida alcanza.
- **Meterlo en `make check`.** Necesitaría red y su veredicto cambiaría con la fecha: el checkpoint
  de cada fase dejaría de ser reproducible y de poder correr sin red.
- **Alertas de Dependabot.** Trabajan por módulo, sin análisis por símbolo. No se descartan como
  complemento futuro.

## Consecuencias

- **Se gana:**
  - ningún PR mergea una vulnerabilidad alcanzable conocida;
  - lo que se publica aparece en un día como máximo;
  - se reproduce local con `make vuln`;
  - se cierra la unidad bloqueada de la auditoría.
- **Se acepta a cambio:**
  - Un PR puede ponerse rojo por un CVE que no tiene nada que ver con su cambio. Se arregla primero
    en un PR propio.
  - `tools.mod` comparte la lista de módulos entre herramientas. Hoy sube `golang.org/x/tools` de
    v0.50.0 a v0.52.0 (y también `x/mod`, `x/sync` y `x/sys`, y suma `x/telemetry`) para
    golangci-lint y sqlc. Cada actualización de govulncheck puede volver a hacerlo; lo controla
    `make lint` en el mismo PR.
  - Si vuln.go.dev no responde, el job falla (falla cerrada) y se vuelve a correr.
  - Las herramientas y npm quedan fuera.
  - Si el repo es público, GitHub desactiva la corrida programada después de 60 días sin actividad
    en el repo, sin avisar. Hay que reactivarla a mano.
