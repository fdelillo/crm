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
- **Dónde viven las tareas**: base de datos "Tareas · CRM" — **PENDIENTE DE CREAR**.
  Hasta que exista, la skill debe frenar y avisarlo (no buscar en otras bases del workspace).
  <!-- Al crearla, reemplazar por: base de datos "Tareas · CRM" (id: xxxxxxxx-...) -->

## Estados

- **Propiedad de estado**: `Status` (tipo `select`)
- **Estado de entrada** (lo que se revisa): `In Review`
- **Estado de salida — aprobado**: `Done`
- **Estado de salida — cambios pedidos**: `Changes Requested`

## Dónde está el código de cada tarea

En orden de preferencia:

1. Propiedad `Rama` de la tarea (texto).
2. Rama del repo cuyo nombre contenga el id corto de la tarea (ej. `CRM-012`).
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
- La columna `Spec` de la tarea, si existe, indica a qué spec pertenece (ej. `001`).
