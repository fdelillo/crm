# CRM para pymes que trabajan por proyecto

Sistema web **sencillo** para gestionar clientes, proyectos, presupuestos y flujo de
caja (clientes, proveedores, empleados y socios). El primer usuario es una carpintería
de aluminio; el diseño es genérico y se adapta a otros rubros mediante configuración.

> Estado: **especificación** (Spec Driven Development). Aún no hay código.

## Documentación

| Documento | Contenido |
|-----------|-----------|
| [Constitución](.specify/memory/constitution.md) | Principios no negociables, stack y convenciones |
| [Visión](docs/vision.md) | Problema, usuarios, alcance del MVP y orden de construcción |
| [Modelo de dominio](docs/modelo-dominio.md) | Entidades, efectos de cada operación, multimoneda |
| [Glosario](docs/glosario.md) | Términos del negocio y su nombre en el código |

## Specs del MVP

| # | Módulo | Spec |
|---|--------|------|
| 001 | Empresas, usuarios y roles | [spec](specs/001-empresas-usuarios/spec.md) |
| 002 | Configuración por empresa | [spec](specs/002-configuracion/spec.md) |
| 003 | Terceros | [spec](specs/003-terceros/spec.md) |
| 004 | Proyectos | [spec](specs/004-proyectos/spec.md) |
| 005 | Presupuestos | [spec](specs/005-presupuestos/spec.md) |
| 006 | Cuentas de dinero | [spec](specs/006-cuentas-dinero/spec.md) |
| 007 | Cuentas corrientes | [spec](specs/007-cuentas-corrientes/spec.md) |
| 008 | Importación de extractos | [spec](specs/008-importacion-extractos/spec.md) |
| 009 | Reportes | [spec](specs/009-reportes/spec.md) |

## Stack

Go (API REST) · PostgreSQL · React + TypeScript (PWA) · contrato OpenAPI · monolito modular.
Detalle y motivos en la [constitución](.specify/memory/constitution.md#stack-tecnológico).

## Flujo de trabajo (GitHub Spec Kit)

Cada funcionalidad sigue este ciclo en su rama `NNN-nombre`:

1. `/speckit.specify`: `spec.md` (qué y por qué). **Hecho para 001–009.**
2. `/speckit.clarify`: resolver ambigüedades de la spec.
3. `/speckit.plan`: `plan.md`, `data-model.md` y `contracts/` (cómo), verificando la constitución.
4. `/speckit.tasks`: `tasks.md`.
5. `/speckit.implement`: implementación con tests.

Para instalar los comandos de Spec Kit en este repo:
`uvx --from git+https://github.com/github/spec-kit.git specify init --here`
(si pregunta, **conservar** la constitución existente).
