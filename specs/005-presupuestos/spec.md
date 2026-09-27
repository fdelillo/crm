# Spec: Presupuestos

**Rama**: `005-presupuestos`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: Un proyecto puede tener varios presupuestos (versiones o alternativas).
Cada uno tiene ítems con medidas, cantidades y precios, IVA opcional, y genera un PDF
con el logo de la empresa que se envía por WhatsApp o email. Al aprobar uno, se fija
el monto del proyecto.

## Escenarios de usuario y pruebas

### Historia 1: Armar un presupuesto (Prioridad: P1)

**Por qué esta prioridad**: es la tarea comercial más frecuente y la que más tiempo ahorra.

**Prueba independiente**: armar un presupuesto de 3 ítems desde el celular y ver el total.

**Escenarios de aceptación**:

1. **Dado** un proyecto, **cuando** se crea un presupuesto, **entonces** se elige su
   moneda (ARS o USD) y queda en estado *borrador* con numeración correlativa
   (ej. PR-0001).
2. **Dado** un presupuesto, **cuando** se agrega un ítem desde el catálogo, **entonces**
   se copian descripción, unidad y precio de referencia, que luego pueden editarse.
3. **Dado** un presupuesto, **cuando** se agrega un ítem libre, **entonces** se ingresan
   descripción, cantidad, unidad y precio unitario.
4. **Dado** un ítem con unidad m², **cuando** se ingresan ancho y alto (en metros),
   **entonces** la superficie se calcula como ancho × alto × cantidad y el subtotal
   como superficie × precio.
5. **Dado** un ítem con precio en otra moneda que la del presupuesto, **entonces** el
   sistema pide un tipo de cambio para convertirlo.
6. **Dado** un presupuesto, **cuando** se aplica un descuento general (porcentaje o
   importe), **entonces** se descuenta del subtotal.

### Historia 2: IVA informativo (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un presupuesto, **cuando** se elige alícuota de IVA (0 %, 10,5 %, 21 % o
   "sin IVA"), **entonces** se muestran neto, IVA y total.
2. **Dado** un presupuesto "sin IVA", **entonces** el PDF solo muestra el total.

### Historia 3: PDF y envío por WhatsApp (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un presupuesto, **cuando** se genera el PDF, **entonces** incluye logo y
   datos de la empresa, datos del cliente, número y fecha, ítems con medidas,
   totales, condiciones comerciales y fecha de validez.
2. **Dado** un presupuesto, **cuando** se toca "Enviar por WhatsApp", **entonces** se
   abre WhatsApp con el teléfono del cliente y un mensaje con el enlace al PDF.
3. **Dado** un enlace compartido, **entonces** es de solo lectura, difícil de adivinar
   y el Administrador puede revocarlo.
4. **Dado** un presupuesto enviado, **entonces** pasa a estado *enviado* con la fecha de envío.
5. **Dado** un presupuesto, **cuando** se elige "Enviar por email", **entonces** se abre
   el cliente de correo con el PDF adjunto o su enlace.

### Historia 4: Versiones y aprobación (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un presupuesto, **cuando** se lo duplica, **entonces** se crea una nueva
   versión en *borrador* con los mismos ítems (ej. "Alternativa con DVH").
2. **Dado** un proyecto con varios presupuestos, **cuando** se aprueba uno, **entonces**
   queda *aprobado*, los demás pasan a *descartado*, el monto del proyecto se fija y la
   cuenta corriente del cliente aumenta por el total.
3. **Dado** un presupuesto aprobado, **entonces** no se puede editar. Para cambiarlo
   se registran adicionales/bonificaciones en el proyecto o se anula la aprobación.
4. **Dado** un presupuesto aprobado sin cobros imputados al proyecto, **cuando** un
   Administrador anula la aprobación indicando un motivo, **entonces** se revierte el
   movimiento de cuenta corriente y el presupuesto vuelve a *enviado*.
5. **Dado** un presupuesto aprobado con cobros, **cuando** se intenta anular la
   aprobación, **entonces** el sistema lo impide hasta anular o reimputar los cobros.
6. **Dado** la aprobación, **entonces** el sistema ofrece mover el proyecto a la
   siguiente etapa.

### Casos borde

- Un presupuesto vencido (fecha de validez pasada) se marca *vencido*, pero puede
  aprobarse si el usuario lo confirma.
- Cambiar los datos de la empresa no altera los PDF ya generados: se guarda una copia
  del PDF al enviarlo.
- Los importes se redondean a 2 decimales por ítem; los totales se calculan sumando
  los ítems ya redondeados.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE permitir múltiples presupuestos por proyecto, con estados
  *borrador*, *enviado*, *aprobado*, *descartado* y *vencido*.
- **FR-002**: El sistema DEBE permitir ítems del catálogo o libres, con descripción,
  cantidad, unidad, ancho y alto opcionales, precio unitario y subtotal.
- **FR-003**: El sistema DEBE calcular automáticamente la superficie para unidades m²
  y la longitud para metro lineal.
- **FR-004**: El sistema DEBE permitir descuento general e IVA opcional.
- **FR-005**: El sistema DEBE generar un PDF con la identidad de la empresa.
- **FR-006**: El sistema DEBE permitir compartir por WhatsApp (enlace `wa.me`) y por email.
- **FR-007**: El sistema DEBE permitir un único presupuesto aprobado por proyecto.
- **FR-008**: La aprobación DEBE generar, de forma atómica, el movimiento en la cuenta
  corriente del cliente en la moneda del presupuesto.
- **FR-009**: El sistema DEBE permitir duplicar presupuestos.

### Entidades clave

- **Presupuesto (Quote)**: número, proyecto, moneda, estado, fecha, validez, descuento,
  alícuota IVA, condiciones, observaciones, totales.
- **Ítem (QuoteItem)**: descripción, ítem de catálogo de origen (opcional), unidad,
  cantidad, ancho, alto, precio unitario, subtotal, orden.

## Criterios de éxito

- **SC-001**: Un presupuesto de 5 ítems se arma y se envía por WhatsApp en menos de
  5 minutos.
- **SC-002**: El PDF se genera en menos de 3 segundos.

## Supuestos

- El PDF no es un comprobante fiscal.
- No hay firma ni aprobación online por parte del cliente en el MVP: la aprobación la
  registra un usuario de la empresa.
