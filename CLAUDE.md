# CLAUDE.md

Proyecto desarrollado con **Spec Driven Development** (GitHub Spec Kit).

- Leer primero `.specify/memory/constitution.md`: sus principios son obligatorios.
- No implementar nada que no esté en una `specs/NNN-*/spec.md` aprobada.
- Documentación en español; código, identificadores y commits en inglés. Usar los
  nombres de `docs/glosario.md`.
- Dinero: enteros en centavos y moneda explícita; nunca `float`. Movimientos
  inmutables (se anulan, no se editan ni se borran). Operaciones atómicas.
- Toda consulta se filtra por `tenant_id`.
- Semántica de cada operación financiera: `docs/modelo-dominio.md` → "Efectos de cada operación".
