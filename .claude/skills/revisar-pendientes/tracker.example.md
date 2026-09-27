# Tablero del proyecto

Plantilla del adaptador que lee `/revisar-pendientes`. Copiala a `.claude/tracker.md` en la
raíz del proyecto y completá los valores reales. Es el único archivo que sabe qué herramienta
de gestión usa el proyecto.

> **Este archivo se versiona: nombra variables, nunca valores.** Los tokens y los ids viven en
> el `.env` del proyecto, que está en `.gitignore`. Acá solo se declara cómo se llaman.

## Identificación

- **Herramienta**: Notion            <!-- Notion | Jira | Trello | GitHub Projects | otro -->
- **Modo de acceso**: api            <!-- api (token del .env) | conector (MCP del desktop app) -->

## Acceso — modo `api`

- **Archivo de variables**: `.env`
- **Variable del token**: `NOTION_TOKEN`
- **Variable de la base de datos**: `NOTION_BASE_DATOS`
- **Variable de la página** (opcional): `NOTION_PAGINA`
- **Versión de la API**: `2022-06-28`

## Acceso — modo `conector`

- **Cómo buscar las tools**: `ToolSearch` con la palabra `notion`
- **Dónde viven las tareas**: base de datos "Tareas" (id: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`)

## Estados

- **Propiedad de estado**: `Estado`
- **Estado de entrada** (lo que se revisa): `En revisión`
- **Estado de salida — aprobado**: `Aprobado`
- **Estado de salida — cambios pedidos**: `Cambios pedidos`

## Dónde está el código de cada tarea

En orden de preferencia:

1. Propiedad `Rama` de la tarea (texto o URL).
2. Rama del repo cuyo nombre contenga el id corto de la tarea.
3. PR abierto que mencione el id de la tarea.

- **Rama base para el diff**: `main`
  <!-- Si la base NO es fija (por ejemplo, cada tarea nace de la rama de la tarea anterior
       de su fase), poné acá el comando que la imprime y la skill lo ejecuta por tarea:
       Comando de base: ./herramientas/tarea fase <n>   -->

## Comentarios

- **Soporta comentarios**: sí
- **Dónde**: comentario en la página de la tarea

## Notas del proyecto

<!-- Cualquier cosa que el revisor tenga que saber: convenciones de nombres de rama,
     qué tareas saltear (por ejemplo las etiquetadas "spike"), quién es el dueño del
     tablero. Una línea cada cosa. -->
