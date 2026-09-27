# Tablero del proyecto

Adaptador que lee `/revisar-pendientes`. Es el único archivo que sabe qué herramienta de
gestión usa el proyecto.

> **Este archivo se versiona: nombra variables, nunca valores.** Si algún día se pasa a modo
> `api`, el token vive en `.env` (ignorado por git) y acá solo se declara su nombre.

## Identificación

- **Herramienta**: Notion
- **Modo de acceso**: conector

## Acceso — modo `conector`

- **Cómo buscar las tools**: `ToolSearch` con la palabra `notion`
- **Dónde viven las tareas**: base de datos "Tareas · CRM", dentro de la página "CRM"
  - Base de datos: `0a5fca8d77744c379165296a005b35fc`
  - Data source: `collection://3bcbed95-c92b-413d-a9ed-2b1da4674467`
  - Usar solo esta base; no buscar tareas en otras bases del workspace.

## Estados

- **Propiedad de estado**: `Status` (tipo `select`)
- **Estado de entrada** (lo que se revisa): `In Review`
- **Estado de salida — aprobado**: `Done`
- **Estado de salida — cambios pedidos**: `Changes Requested`

## Dónde está el código de cada tarea

En orden de preferencia:

1. Propiedad `Rama` de la tarea (texto).
2. Rama del repo cuyo nombre contenga el `ID` de la tarea (ej. `CRM-012`).
3. PR abierto en `fdelillo/crm` que mencione el id de la tarea.

- **Rama base para el diff**: `main`

## Comentarios

- **Soporta comentarios**: sí
- **Dónde**: comentario en la página de la tarea

## Notas del proyecto

- En sesiones en la nube no hay CLI `gh`: los PR se buscan con las tools de GitHub
  (`mcp__github__*`, vía `ToolSearch` con la palabra `github`).
- El revisor contrasta cada tarea contra `specs/NNN-*/` (spec, plan, tasks) y la constitución
  (`.specify/memory/constitution.md`).
- Propiedades de la base: `Tarea` (título), `ID` (id corto, ej. `CRM-012`), `Status`,
  `Spec` (ej. `001 · Empresas y usuarios`), `Fase` (fase de `tasks.md`), `Orden`, `Rama`,
  `Cierra` (FR/US que cierra). Estados: Pending, In Progress, In Review,
  Changes Requested, Done, Blocked.
- `Spec` y `Cierra` indican contra qué `specs/NNN-*/` y requisitos revisar la tarea.
