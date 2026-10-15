# AGENTS.md

Instrucciones para Codex. Las reglas del proyecto son las mismas para Codex y para Claude Code,
y están escritas una sola vez:

- **Reglas generales**: `CLAUDE.md` (leelo antes de cualquier tarea) y
  `.specify/memory/constitution.md`, que prevalece sobre todo.
- **Agentes**: `.claude/agents/*.md`. Codex los usa a través de `.codex/agents/*.toml`, que se
  generan con `make codex-agents` y no se editan a mano. `make lint` falla si quedan
  desincronizados.

## Quién hace qué

Este hilo orquesta: reparte el trabajo entre los agentes del proyecto y le reporta al usuario.
No hace por su cuenta lo que le corresponde a un agente.

| Trabajo | Agente | Nota |
|---|---|---|
| Diseño de backend: `plan.md`, `research.md`, `data-model.md`, `contracts/openapi.yaml`, sección Backend de `tasks.md`, ADR | `backend-architect` | No escribe código |
| Diseño de frontend: `ui.md`, sección Frontend de `tasks.md` | `frontend-architect` | No escribe código |
| Implementar **una** fase ya aprobada | `backend-developer` / `frontend-developer` | Modo de `.claude/dev-mode` (`completo`) |
| Revisar un diff, una rama, un PR o una fase | `code-reviewer` | No edita: reporta |

### Cómo lanzar un agente

Si tu herramienta para lanzar subagentes deja elegir un agente por nombre, elegí el de la tabla.
Si no lo deja (por ejemplo, si `spawn_agent` solo recibe `task_name` y `message`), lanzalo así:

1. Leé `.codex/agents/<agente>.toml`.
2. Lanzá un subagente **sin heredar el contexto** de este hilo (`fork_turns: "none"`), con el
   `model_reasoning_effort` del TOML como esfuerzo de razonamiento y `task_name` igual al nombre
   del agente, con guiones bajos.
3. El mensaje empieza con el texto de `developer_instructions` del TOML, completo, y sigue con
   la tarea.

Arrancar sin el contexto de este hilo es parte de la regla, no un detalle: el agente tiene que
trabajar con sus propias instrucciones y con el repositorio, no con lo que este hilo ya decidió.
En una revisión, eso es lo que la vuelve independiente.

## Reglas del flujo

- **El diseño lo aprueba el usuario.** Ningún agente cambia `spec.md` sin su aprobación. Los
  documentos de diseño solo los cambian los arquitectos, y el cambio se aplica cuando el usuario
  lo aprueba. Un desarrollador que encuentra un hueco o un error en el diseño frena y lo reporta:
  no lo resuelve en el código ni edita el texto de diseño.
- **Una fase por invocación** de un desarrollador, con el ciclo TDD de `tasks.md` y el checkpoint
  de la fase en verde (`make check`, más los comandos propios de la fase).
- **Toda fase pasa por `code-reviewer`** antes de darla por cerrada. Los hallazgos de diseño
  vuelven al arquitecto; los de código, al desarrollador.
- **Nombres por funcionalidad**, nunca por fase, tarea, revisión ni id de caso. La trazabilidad
  va en un comentario (`// T-B1005 (Q1–Q8): …`). La regla completa está en la sección 0 de cada
  agente y en las convenciones de `tasks.md`.
- **Git**: una rama por fase o por cambio, creada desde `main`, con un PR. El merge lo hace el
  usuario.
- **Skills de UI**: para el diseño visual del frontend, leé
  `.claude/skills/ui-ux-pro-max/SKILL.md`.
