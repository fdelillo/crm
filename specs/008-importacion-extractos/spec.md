# Spec: Importación de extractos bancarios

**Rama**: `008-importacion-extractos`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: Para no cargar a mano cada movimiento bancario y detectar olvidos, el
Administrador sube el extracto del banco o billetera (CSV o Excel) y concilia cada
línea con los movimientos del sistema.

## Escenarios de usuario y pruebas

### Historia 1: Subir un extracto (Prioridad: P1)

**Por qué esta prioridad**: sin importación no hay conciliación.

**Prueba independiente**: subir un CSV de 20 líneas a una cuenta bancaria y ver las
líneas listas para conciliar.

**Escenarios de aceptación**:

1. **Dado** una cuenta de dinero de tipo banco o billetera, **cuando** se sube un
   archivo CSV, XLS o XLSX, **entonces** el sistema muestra una vista previa de las
   primeras filas.
2. **Dado** la primera importación de una cuenta, **cuando** el usuario indica qué
   columna es fecha, descripción e importe (o débito/crédito por separado) y el
   formato de fecha y decimales, **entonces** el sistema guarda ese mapeo para las
   próximas importaciones de esa cuenta.
3. **Dado** un archivo con líneas ya importadas antes, **entonces** esas líneas se
   detectan como duplicadas (misma fecha, importe y descripción) y no se vuelven a importar.
4. **Dado** un archivo con filas inválidas (fecha o importe ilegible), **entonces** se
   informan y se omiten, sin impedir importar el resto.

### Historia 2: Conciliar líneas (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** una línea importada, **cuando** existe un movimiento vigente en esa cuenta
   con el mismo importe y fecha cercana (±3 días) sin conciliar, **entonces** el
   sistema lo sugiere para conciliar.
2. **Dado** una sugerencia, **cuando** el usuario la confirma, **entonces** la línea y
   el movimiento quedan conciliados.
3. **Dado** una línea sin movimiento, **cuando** el usuario elige "Crear movimiento",
   **entonces** se abre el formulario correspondiente (gasto directo, cobro, pago,
   etc.) precargado con fecha, importe y descripción.
   Si la línea corresponde a un cheque propio pendiente del mismo importe, el sistema
   sugiere marcarlo como *debitado* (ver [010](../010-cheques/spec.md)).
4. **Dado** una línea irrelevante o ya registrada de otra forma, **cuando** el usuario
   elige "Ignorar", **entonces** se marca como ignorada.
5. **Dado** un extracto, **entonces** se ve el avance: líneas conciliadas, pendientes e ignoradas.

### Historia 3: Reglas simples de categorización (Prioridad: P3)

**Escenarios de aceptación**:

1. **Dado** líneas cuya descripción contiene un texto (ej. "COMISION", "IMP DEB CRED"),
   **cuando** el usuario crea una regla "descripción contiene X → gasto directo con
   categoría Y", **entonces** las líneas nuevas que coinciden se proponen con esa
   categoría para confirmar en lote.

### Casos borde

- Si la moneda del extracto no coincide con la de la cuenta, la importación se rechaza.
- Anular un movimiento conciliado devuelve la línea a *pendiente*.
- Un movimiento solo puede conciliarse con una línea.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE importar archivos CSV, XLS y XLSX de hasta 5 MB.
- **FR-002**: El sistema DEBE permitir mapear columnas y guardar el mapeo por cuenta.
- **FR-003**: El sistema DEBE detectar líneas duplicadas entre importaciones.
- **FR-004**: El sistema DEBE sugerir conciliaciones por importe y fecha cercana.
- **FR-005**: El sistema DEBE permitir crear movimientos desde una línea, conciliar e ignorar.
- **FR-006**: Importar un extracto NO DEBE crear movimientos automáticamente sin
  confirmación del usuario.

### Entidades clave

- **Extracto (BankStatement)**: cuenta, archivo, fecha de carga, usuario.
- **Línea de extracto (StatementLine)**: fecha, descripción, importe, estado
  (pendiente/conciliada/ignorada), movimiento conciliado, huella para duplicados.
- **Mapeo de columnas (StatementMapping)**: cuenta, columnas y formatos.
- **Regla de categorización (MatchingRule)**: texto, tipo de operación, categoría.

## Criterios de éxito

- **SC-001**: Conciliar un extracto mensual de 100 líneas lleva menos de 15 minutos.

## Supuestos

- No se conecta con APIs bancarias en el MVP.
- La importación es exclusiva para Administradores.
