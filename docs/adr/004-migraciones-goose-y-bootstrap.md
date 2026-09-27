# ADR-004: Migraciones con goose embebidas en el binario, y bootstrap de roles fuera de goose

**Status**: Accepted (goose: decisión del usuario; separación del bootstrap: propuesta del arquitecto)
**Fecha**: 2026-09-27
**Origen**: spec 001

## Contexto

El esquema evoluciona con cada spec. Además, el aislamiento por rol de empresa (ADR-005)
necesita **roles de PostgreSQL**, que son objetos **del clúster** (no de una base) y cuya
creación exige `CREATEROLE`. Mezclar ambas cosas en un único mecanismo obligaría a que el rol que
corre migraciones pueda crear roles arbitrarios.

## Decisión

1. **goose v3** (`github.com/pressly/goose/v3`) con migraciones SQL versionadas en
   `db/migrations/`, embebidas con `//go:embed` y ejecutadas por el subcomando
   `crm migrate {up|down|status}` usando `goose.NewProvider` con ese `embed.FS`.
2. Las migraciones corren con credenciales de **`crm_owner`** (`DATABASE_MIGRATION_URL`), dueño
   del esquema, **nunca** con el rol de runtime (`crm_app`). Se ejecutan como paso de despliegue,
   no al arrancar `serve`. `/readyz` responde 503 si la versión en la base no es la que espera el
   binario.
3. La tabla de versiones de goose vive en el esquema `public` (sin privilegios para roles de
   runtime).
4. Los **roles de clúster** y la base se crean con un script SQL idempotente en `db/bootstrap/`,
   que ejecuta un DBA una vez por clúster (en desarrollo: script de inicio del contenedor; en
   tests: el harness). Crea `crm_owner`, `crm_app`, `crm_tenant`, `crm_auth`, `crm_worker`,
   `crm_signup`, `crm_provisioner` con sus membresías.
5. `crm_owner` puede hacer `SET ROLE crm_provisioner` para que una migración cree o modifique la
   función de aprovisionamiento de roles (dueña `crm_provisioner`), pero no tiene `CREATEROLE`
   propio.
6. Cada migración tiene `Down`. Una migración que necesite leer o reescribir datos de varias
   empresas (*backfill*) desactiva `FORCE ROW LEVEL SECURITY` en la tabla **dentro de la misma
   migración** y lo reactiva antes de terminar, de forma explícita y revisable.
7. Toda tabla nueva de empresa sigue el checklist de `data-model.md` de 001 (RLS forzada,
   política `tenant_isolation`, FKs compuestas, `UPDATE`/`DELETE` concedidos explícitamente). Los
   tests de catálogo (T-B107..T-B109) la verifican sin escribir tests nuevos.

## Fundamento

- goose usa SQL plano con anotaciones `-- +goose Up/Down` en un solo archivo, se integra como
  librería (`Provider`) y admite `embed.FS`: el binario desplegado trae sus propias migraciones y
  no puede desincronizarse de ellas.
- Separar credenciales: el rol de runtime no puede alterar el esquema, y el de migraciones no se
  usa para atender tráfico.
- Los roles de clúster no pertenecen a una base: ponerlos en migraciones haría que restaurar o
  clonar una base intente recrear roles globales, y exigiría `CREATEROLE` al dueño del esquema.
- Desactivar `FORCE` dentro de la migración deja el bypass acotado a esa transacción y visible en
  el historial.

## Alternativas consideradas

- **golang-migrate**: archivos `up`/`down` separados y un estado "dirty" que hay que limpiar a
  mano tras un fallo.
- **Atlas**: migraciones declarativas con diff y lint; más conceptos para el equipo y partes con
  licencia comercial.
- **tern**: menos adopción y documentación.
- **Migraciones al arrancar `serve`**: descartado porque el binario de runtime necesitaría las
  credenciales de dueño.
- **Roles creados por migraciones**: descartado por lo explicado (privilegios y alcance de
  clúster).
- **Dar `BYPASSRLS` a `crm_owner` para los backfills**: descartado; un rol con bypass permanente
  es un riesgo mayor que un `NO FORCE` explícito y temporal.

## Consecuencias

- Ganás: binario autocontenido, credenciales separadas, bootstrap reproducible.
- Aceptás: dos mecanismos (bootstrap + goose) que hay que documentar en el README; el bootstrap
  requiere un DBA con `CREATEROLE` una vez por clúster; cambiar la función de aprovisionamiento
  requiere que `crm_owner` pueda asumir `crm_provisioner` (el rol de migraciones puede, en la
  práctica, crear roles: se acepta porque ya es el rol más privilegiado del sistema).
