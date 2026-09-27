# CLAUDE.md

Proyecto desarrollado con **Spec Driven Development** (GitHub Spec Kit).

- Leer primero `.specify/memory/constitution.md`: sus principios son obligatorios.
- No implementar nada que no esté en una `specs/NNN-*/spec.md` aprobada.
- Documentación en español; código, identificadores y commits en inglés. Usar los
  nombres de `docs/glosario.md`.
- Dinero: enteros en centavos y moneda explícita; nunca `float`. Movimientos
  inmutables (se anulan, no se editan ni se borran). Operaciones atómicas.
- Operaciones entre monedas: se guardan ambos importes informados por el usuario;
  el tipo de cambio no se persiste.
- Toda consulta se filtra por `tenant_id`.
- Semántica de cada operación financiera: `docs/modelo-dominio.md` → "Efectos de cada operación".

## Agentes y skills del proyecto (`.claude/`)

Flujo por spec (`specs/NNN-slug/`):

| Paso | Quién | Produce |
|------|-------|---------|
| Plan técnico backend | `backend-architect` | `plan.md`, `research.md`, `data-model.md`, `contracts/openapi.yaml`, sección Backend de `tasks.md`, ADR en `docs/adr/` |
| Plan técnico frontend | `frontend-architect` | `ui.md`, sección Frontend de `tasks.md` |
| Implementación (una fase por vez) | `backend-developer` / `frontend-developer` | Código y tests; modo en `.claude/dev-mode` (`completo`) |
| Revisión | `code-reviewer` | Hallazgos por severidad (no edita) |

Skills: `ui-ux-pro-max` (diseño de UI), `archify` (diagramas HTML), `graphify` (grafo del
código, solo a pedido), `revisar-pendientes` (revisa tareas del tablero de Notion según
`.claude/tracker.md`).
