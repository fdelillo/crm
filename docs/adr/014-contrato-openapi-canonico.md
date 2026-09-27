# ADR-014: Contrato OpenAPI 3.1 canónico, handlers escritos a mano y validación en tests

**Status**: Proposed
**Fecha**: 2026-09-27
**Origen**: spec 001

## Contexto

La constitución define OpenAPI como fuente de verdad entre frontend y backend y exige tests de
contrato (principio VI). Hay que decidir si el servidor se genera desde el contrato o se escribe a
mano y se verifica, y dónde vive el contrato cuando hay varias specs.

## Decisión

- Cada spec tiene su contrato en `specs/NNN-slug/contracts/openapi.yaml`, **OpenAPI 3.1**, y es
  **canónico**: si un documento de diseño lo contradice, manda el YAML.
- Los componentes compartidos (`Problem`, `ValidationProblem`, `ErrorCode`, `Role`,
  `Permission`) viven en el contrato de 001; las specs siguientes los referencian con `$ref`.
- **Servidor**: handlers y DTOs escritos a mano en `internal/<módulo>/http.go`. No se genera
  código de servidor.
- **Verificación**: todos los tests HTTP validan el request y la response contra el contrato con
  `github.com/pb33f/libopenapi-validator`. Un test además compara las rutas registradas en chi con
  los `paths` del contrato (ninguna ruta sin contrato, ningún `path` sin ruta).
- **Frontend**: los tipos del cliente se derivan del contrato (herramienta a decidir en `ui.md`).
  Si hace falta un único archivo con todas las specs, se genera con un paso de *bundle*; el
  resultado es derivado, nunca se edita.
- JSON en `snake_case`; `additionalProperties: false` en los requests (el servidor rechaza campos
  desconocidos con `400`).

## Fundamento

- Escribir handlers a mano mantiene el código legible para un equipo que está aprendiendo Go, y
  la validación en tests da la misma garantía de conformidad sobre lo que realmente responde el
  servidor.
- El soporte de OpenAPI 3.1 en los generadores de servidor para Go es reciente (oapi-codegen lo
  anunció como soporte inicial en su v2.8.0); los validadores declaran soporte de 3.0, 3.1 y 3.2.
- Comparar rutas contra `paths` detecta endpoints sin documentar.

## Alternativas consideradas

- **Generar el servidor con oapi-codegen** (interfaz *strict server* para chi): el compilador
  obliga a cumplir el contrato, pero el soporte de 3.1 es inicial y el código generado agrega una
  capa que el equipo debe entender. Reevaluable cuando madure.
- **Generar el contrato desde el código** (anotaciones): invierte la fuente de verdad; el contrato
  dejaría de ser un diseño previo.
- **OpenAPI 3.0**: más soporte de herramientas, pero nulabilidad y JSON Schema menos expresivos;
  3.1 alinea el contrato con JSON Schema estándar.
- **Un contrato único para todo el proyecto**: conflictos entre specs desarrolladas en paralelo;
  se prefiere uno por spec con componentes compartidos.

## Consecuencias

- Ganás: contrato legible y canónico; conformidad verificada en cada test HTTP.
- Aceptás: la conformidad se verifica en tests, no en compilación (una respuesta no cubierta por
  ningún test podría divergir; la cobertura de rutas lo acota); mantener DTOs a mano.
