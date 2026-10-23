# ADR-028: Configuración por empresa referenciable: sin borrado físico, claves de sistema estables y alta por migración

**Status**: Accepted (aprobado por el usuario con el plan de 002, 2026-10-23)
**Fecha**: 2026-10-23
**Origen**: spec 002 (`specs/002-configuracion/plan.md`, DD-6, DD-8); lo usan las specs 004 a 010

## Contexto

La spec 002 crea los catálogos de configuración de cada empresa: etapas de proyecto, categorías de
ingresos y egresos, ítems de catálogo. Las specs siguientes los van a **referenciar** con FK: los
proyectos y su historial de etapas (004), los ítems de presupuesto (005), los movimientos de caja,
compras y cargos (006, 007), las reglas de categorización (008) y los reportes por categoría (009).
La spec pide que renombrar no altere los datos históricos "que la referencian por identificador", y
que una categoría con movimientos no se pueda eliminar.

Además, algunas operaciones automáticas necesitan **una categoría concreta** que el usuario puede
renombrar: "Cobros de clientes" para un cobro (007), "Pagos a proveedores" para un pago, y 006 ya
anticipa "Ajuste de arqueo". Las specs que necesitan esas categorías todavía no existen, así que el
mecanismo para agregarlas tiene que quedar decidido ahora.

Fuerzas: integridad referencial entre specs; que el usuario pueda limpiar su configuración; que las
operaciones automáticas no dependan del nombre que eligió el usuario; que agregar una pieza "de
sistema" en una spec futura sea un procedimiento conocido, no un diseño nuevo cada vez.

## Decisión

1. **Ninguna fila de configuración referenciable se borra físicamente.** Ningún rol de runtime
   tiene `DELETE` sobre esas tablas. El retiro es lógico:
   - lo que el usuario **elimina** sin vuelta atrás (etapas) lleva `deleted_at` y deja de existir
     para la interfaz y la API (`404`), pero sigue resolviendo por id para el historial;
   - lo que el usuario **archiva** (categorías, ítems de catálogo) lleva `archived_at`, deja de
     ofrecerse y se puede restaurar.
2. Toda tabla de configuración referenciable tiene `UNIQUE (tenant_id, id)` y las specs que la
   referencian usan **FK compuestas** con `tenant_id` (INV-07 de 001). Sus filas se bloquean solo con
   `FOR NO KEY UPDATE` (DD-40 de 001).
3. Una pieza **de sistema** (la que una operación automática necesita encontrar) se identifica por
   una **clave estable en inglés** (`system_key`), nunca por nombre ni por id. La clave y el tipo
   son inmutables (privilegio `UPDATE` por columna) y la base impide archivarla (`CHECK`). Hay como
   mucho una por clave y empresa (índice único parcial). El usuario la puede renombrar.
4. **Una spec que necesita una pieza de sistema nueva la agrega con su propia migración**, que en una
   sola transacción: amplía el `CHECK` del enumerado de claves (y el de tipo por clave, si existe),
   desactiva `FORCE ROW LEVEL SECURITY` en las tablas que lee y escribe, inserta la fila en **todas**
   las empresas que no la tienen, y vuelve a activar `FORCE` (ADR-004 §6). En el mismo cambio suma la
   clave al sembrado del registro.

## Fundamento

- Con borrado físico, una FK de 004 o 006 impediría borrar (error confuso) o, con `ON DELETE SET
  NULL`/`CASCADE`, perdería historial financiero, que el principio IV protege.
- Una regla única ("nunca se borra") evita que cada spec decida su propia variante y que la
  regla dependa de saber si hay referencias en tablas de otros módulos (que un módulo no puede leer,
  ADR-001).
- Buscar por clave hace que renombrar sea inofensivo, que es justo lo que la spec permite para las
  categorías de sistema.
- Agregar la pieza en la migración (y no "al vuelo" en el primer uso) mantiene la invariante "toda
  empresa tiene una por clave" desde el despliegue, sin estados intermedios ni carreras.
- Que la base sostenga lo de sistema (`CHECK`, índice, privilegios) protege a specs que van a
  depender de estas filas sin poder verificar el código de otra.

## Alternativas consideradas

- **Borrado físico con `ON DELETE RESTRICT`** (borrar solo si nada la referencia): obliga a la
  interfaz a explicar un error que depende de tablas de otras specs, y cambia de comportamiento
  según la antigüedad de la empresa.
- **Borrado físico con `ON DELETE SET NULL`**: pierde a qué categoría o etapa pertenecía un dato
  histórico (reportes por categoría de 009 incorrectos).
- **Pieza de sistema identificada por nombre o por un booleano `is_system`**: renombrarla rompe las
  operaciones automáticas, o no se distingue cuál de varias es cuál.
- **Tabla separada para las piezas de sistema**: los referenciadores necesitarían dos FK posibles o
  una unión en cada consulta.
- **Crear la pieza de sistema la primera vez que se usa** (`INSERT … ON CONFLICT` en la operación):
  una escritura de configuración escondida en una operación financiera, con concurrencia que
  resolver, y la invariante solo vale "después del primer uso".
- **Triggers que impidan borrar o archivar**: la convención del proyecto es sin triggers; los
  privilegios y `CHECK` alcanzan.

## Consecuencias

- Ganás: referencias siempre resolubles entre specs; una sola forma de retirar configuración;
  piezas de sistema que sobreviven a cualquier renombre; un procedimiento conocido para agregarlas.
- Aceptás: las tablas acumulan filas retiradas (volumen despreciable); las vistas de historial
  tienen que mostrar elementos eliminados o archivados ("(eliminada)") y tolerar nombres repetidos
  entre una activa y una retirada; agregar una pieza de sistema exige una migración con *backfill*
  y alterar `CHECK`; no hay forma de borrar una categoría creada por error (se archiva).
- Primer uso previsto fuera de 002: la categoría de sistema "Ajuste de arqueo"
  (`cash_count_adjustment`), que agrega la spec 006.
- Si alguna vez hiciera falta borrar físicamente (p. ej. por un pedido legal sobre datos
  personales, que la configuración no tiene), se escribe un ADR que reemplace a este.
