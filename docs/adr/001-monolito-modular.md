# ADR-001: Estructura del monolito modular (layout, módulos por dominio y fronteras)

**Status**: Accepted (aprobado por el usuario con el plan de 001, 2026-09-29)
**Fecha**: 2026-09-27
**Origen**: spec 001 (`specs/001-empresas-usuarios/plan.md` §4.1 y §11)

> **Nota 2026-09-29 (detalle; no cambia la decisión)**: el paquete `web` (SPA embebida con
> `go:embed`, ADR-019) vive en la **raíz del repo, fuera de `internal/`**, porque `go:embed` solo
> ve archivos bajo el directorio del paquete y el build del frontend deja `dist/` en `web/dist`.
> Sigue sin ser importable desde fuera del binario en la práctica: `depguard` restringe su
> import a `internal/app` y `cmd/crm`. `internal/app` arma además el **mux raíz** que reparte
> `/api/`, `/healthz`, `/readyz` y la SPA (DD-22 del plan de 001, hallazgo H-1). No se crea un
> ADR nuevo porque la organización por dominio, las reglas de dependencia y el composition root
> no cambian: se agrega un paquete de assets sin lógica de dominio.

## Contexto

La constitución fija un **monolito modular**: un solo servicio desplegable, módulos separados
por dominio, sin microservicios. Falta decidir cómo se ve eso en un repositorio Go: dónde vive
cada módulo, qué puede importar a qué, y cómo se impide que con el tiempo todo dependa de todo.
Hay 10 specs en el MVP; cada una agrega un dominio (empresas, configuración, terceros,
proyectos, dinero…). El equipo tiene nivel intermedio en Go.

Fuerzas: encapsulamiento real (que un módulo no lea tablas de otro), simplicidad (pocas capas
por módulo), y que las fronteras se verifiquen automáticamente y no por disciplina.

## Decisión

1. Un único módulo Go (`github.com/fdelillo/crm`), un binario (`cmd/crm`) con subcomandos
   `serve`, `migrate` y comandos de operación.
2. Todo el código de aplicación bajo `internal/`, organizado **por dominio**:
   - `internal/<módulo>/` por dominio (`tenant`, `identity`, luego `settings`, `party`,
     `project`…). Dentro: tipos y reglas, `service.go` (casos de uso), `http.go` (handlers y
     DTOs), `store/` (queries SQL + código sqlc).
   - `internal/authz`: roles y permisos (transversal a todos los dominios).
   - `internal/platform/*`: piezas técnicas sin dominio (db, httpx, outbox, mailer,
     objectstore, audit, password, config…).
   - `internal/app`: *composition root*: construye dependencias y arma el router. Es el único
     paquete que importa todo.
3. Reglas de dependencia, verificadas con `golangci-lint` + `depguard` en `make lint`:
   - `platform/*` no importa ningún módulo de dominio ni `authz`.
   - Un módulo **nunca** importa el `store` de otro módulo (no lee tablas ajenas).
   - Las dependencias entre módulos son en un solo sentido y se declaran en el plan de cada
     spec (en 001: `tenant → identity`; `identity` no importa `tenant`).
   - El consumidor declara la interfaz chica que necesita del otro módulo ("aceptá interfaces,
     devolvé structs").
4. Una operación que abarca dos módulos comparte **la misma transacción**: los métodos que
   participan de una transacción ajena reciben `db.Tx` explícito como parámetro.

## Fundamento

- En Go, el paquete es la unidad de encapsulamiento: lo no exportado es invisible afuera.
  Agrupar por dominio permite que casi todo quede sin exportar; agrupar por capa técnica obliga a
  exportar todo para cruzar de `handlers` a `services` a `repositories`.
- `internal/` hace que el compilador impida importar esos paquetes desde fuera del módulo.
- `depguard` convierte las fronteras en errores de lint: una violación rompe CI, no depende de
  que alguien la vea en la revisión.
- Pasar `db.Tx` explícito (en vez de esconder la transacción en el `context`) hace visible en la
  firma qué operaciones son parte de una transacción mayor, lo que importa para la atomicidad
  (principio IV).
- Las interfaces del lado del consumidor evitan ciclos de import y permiten sustituir la
  dependencia en tests del consumidor.

## Alternativas consideradas

- **Capas técnicas globales** (`handlers/`, `services/`, `repositories/`): descartada porque
  obliga a exportar todo, dispersa cada feature en varias carpetas y no impide que un servicio
  consulte tablas de otro dominio.
- **Hexagonal completa por módulo** (dominio, aplicación, puertos, adaptadores en paquetes
  separados): descartada por costo cognitivo; para CRUD con reglas moderadas son 3–4 paquetes
  por módulo sin beneficio proporcional. Se conserva la idea de **puerto** solo donde hay
  infraestructura externa sustituible (email, archivos).
- **Un módulo Go por dominio (multi-módulo con `go.work`)**: descartada; versionado y
  dependencias entre módulos sin necesidad en un único binario.
- **Microservicios**: prohibidos por la constitución y sin requisito que los justifique.
- **Transacción implícita en el `context`**: descartada porque oculta qué funciones escriben
  dentro de qué transacción; es fácil abrir una segunda transacción sin darse cuenta.

## Consecuencias

- Ganás: fronteras verificadas en CI; cada spec nueva agrega un directorio sin tocar los
  demás; lo interno de un dominio no se filtra.
- Aceptás: algo de repetición entre módulos (cada uno mapea sus structs sqlc a tipos de
  dominio y a DTOs); `internal/app` crece con cada módulo (es plomería, no lógica).
- Si dos módulos necesitan dependerse mutuamente, es señal de que la frontera está mal
  trazada: se resuelve moviendo el concepto o con una interfaz del lado del consumidor, nunca
  relajando `depguard`.
