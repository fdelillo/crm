# Spec: Cheques

**Rama**: `010-cheques`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: La empresa recibe cheques de clientes (físicos o e-cheq, a menudo
diferidos) y los deposita o los endosa a proveedores. También emite cheques propios
para pagar. Necesita saber qué cheques tiene en cartera, cuándo se pueden cobrar,
a quién se endosaron, cuáles propios están por debitarse y qué pasa si uno rebota.

## Clarificaciones

### Sesión 2026-09-27

- P: ¿Qué cheques se manejan? → R: De terceros (recibidos de clientes) y propios
  (emitidos por la empresa).
- P: ¿Qué operaciones entran en el MVP? → R: Recibir como cobro, depositar o cobrar,
  endosar a proveedor y rechazo.
- P: ¿Se distinguen físicos y e-cheq? → R: Sí, solo como dato; se operan igual.
- P: ¿Cuándo se construye? → R: En el MVP, después de 006 y 007.

## Modelo

Los cheques se apoyan en dos tipos especiales de
[cuenta de dinero](../006-cuentas-dinero/spec.md), para reutilizar los saldos, las
transferencias y las anulaciones que ya existen:

| Cuenta | Qué representa | Saldo |
|--------|----------------|-------|
| **Cartera de cheques** (una por moneda) | Cheques de terceros que la empresa tiene en su poder | Suma de los cheques *en cartera* |
| **Cheques propios a debitar** (una por cuenta bancaria) | Cheques emitidos que el banco todavía no debitó | Negativo: suma de los cheques propios *emitidos* pendientes |

Cada cheque es además un registro propio con sus datos y su estado.
Los efectos de cada operación están en la
[tabla de efectos](../../docs/modelo-dominio.md#efectos-de-cada-operación).

## Escenarios de usuario y pruebas

### Historia 1: Recibir un cheque como cobro (Prioridad: P1)

**Por qué esta prioridad**: es la forma de pago más habitual de muchos clientes.

**Prueba independiente**: registrar un cobro con un cheque diferido y verlo en la
cartera con su fecha de pago.

**Escenarios de aceptación**:

1. **Dado** un cobro de cliente, **cuando** se elige "Cheque" como medio, **entonces**
   se cargan número, banco, librador (nombre y CUIT), fecha de emisión, fecha de pago,
   importe, moneda y tipo (físico / e-cheq), y el cheque entra a la cartera en estado
   *en cartera*.
2. **Dado** ese cobro, **entonces** la cuenta corriente del cliente y el saldo del
   proyecto bajan igual que en cualquier cobro (ver
   [007](../007-cuentas-corrientes/spec.md)), incluso si el cheque es diferido.
3. **Dado** un cliente que paga con varios cheques, **cuando** registra el cobro,
   **entonces** puede cargar varios cheques y, opcionalmente, efectivo o transferencia
   en la misma operación.
4. **Dado** un cheque en una moneda distinta de la del proyecto, **entonces** aplica
   la regla de los dos importes (Constitución, principio IV).
5. **Dado** un Operador, **entonces** puede recibir cheques como cobro, pero no ve la
   cartera.

### Historia 2: Consultar la cartera (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** cheques en cartera, **cuando** el Administrador abre la cartera, **entonces**
   los ve ordenados por fecha de pago, con total por moneda y marcados como
   *disponible* (fecha de pago cumplida) o *diferido*.
2. **Dado** cualquier cheque, **cuando** se abre, **entonces** se ve su historial:
   de quién se recibió, a qué proyecto se imputó, dónde se depositó o a quién se endosó.
3. **Dado** un cheque, **cuando** se busca por número, librador o importe,
   **entonces** se encuentra aunque ya no esté en cartera.

### Historia 3: Depositar o cobrar un cheque (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un cheque en cartera, **cuando** se deposita en una cuenta bancaria de la
   misma moneda, **entonces** sale de la cartera, entra a la cuenta bancaria y queda
   en estado *depositado*. Es una transferencia: no es ingreso ni gasto.
2. **Dado** un cheque cobrado por ventanilla, **cuando** se registra, **entonces** entra
   a una cuenta de efectivo y queda *cobrado*.
3. **Dado** un cheque con fecha de pago futura, **cuando** se intenta depositar,
   **entonces** el sistema lo advierte, pero permite continuar.
4. **Dado** varios cheques, **cuando** se seleccionan juntos, **entonces** se pueden
   depositar en una sola operación.

### Historia 4: Endosar un cheque a un proveedor (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un proveedor con deuda y cheques en cartera, **cuando** se registra un pago
   eligiendo uno o más cheques, **entonces** los cheques salen de la cartera, quedan
   *endosados* a ese proveedor y la deuda baja por su importe.
2. **Dado** un pago, **entonces** puede combinar cheques endosados, cheques propios y
   dinero de otras cuentas.

### Historia 5: Pagar con cheque propio (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** una cuenta bancaria, **cuando** se paga a un proveedor con un cheque
   propio (número, fecha de emisión, fecha de pago, importe, tipo), **entonces** la
   deuda con el proveedor baja, el cheque queda *emitido* y se registra en "Cheques
   propios a debitar" de esa cuenta bancaria. El saldo del banco todavía no cambia.
2. **Dado** un cheque propio emitido, **cuando** se marca como *debitado* (a mano o al
   conciliar el extracto, ver [008](../008-importacion-extractos/spec.md)),
   **entonces** baja el saldo de la cuenta bancaria y se cancela el pendiente.
3. **Dado** los cheques propios emitidos, **entonces** se ve un listado por fecha de
   pago con el total a debitar por cuenta bancaria, para prever fondos.

### Historia 6: Rechazo de un cheque (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un cheque de terceros rechazado, **cuando** el Administrador registra el
   rechazo con fecha y motivo, **entonces** el cheque queda *rechazado* y el cliente
   vuelve a deber su importe, imputado al mismo proyecto.
2. **Dado** que el cheque estaba *depositado*, **entonces** además sale su importe de
   la cuenta bancaria donde se depositó.
3. **Dado** que el cheque estaba *endosado*, **entonces** además la empresa vuelve a
   deberle el importe al proveedor.
4. **Dado** que el cheque seguía *en cartera*, **entonces** además sale de la cartera.
5. **Dado** un rechazo con gastos bancarios, **cuando** se informan, **entonces** se
   registran como gasto directo, que opcionalmente se puede cargar al cliente como
   adicional del proyecto.
6. **Dado** un cheque propio rechazado, **cuando** se registra, **entonces** se cancela
   el pendiente a debitar y la empresa vuelve a deberle al proveedor.

### Casos borde

- Un rechazo **no es una anulación**: el cobro o pago original sigue vigente y el
  rechazo es una operación nueva que lo compensa. Anular se reserva para errores de
  carga.
- No se puede anular el cobro que originó un cheque si el cheque ya se depositó o se
  endosó; primero hay que anular esas operaciones.
- Un cheque solo puede estar en un estado a la vez y cada transición queda en su historial.
- El número de cheque no es único globalmente (bancos distintos). Se advierte si se
  repite la combinación banco + número.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE registrar cheques de terceros y propios con: número,
  banco, librador y CUIT (terceros), fecha de emisión, fecha de pago, importe, moneda,
  tipo (físico / e-cheq) y estado.
- **FR-002**: Estados de cheques de terceros: *en cartera*, *depositado*, *cobrado*,
  *endosado*, *rechazado*. Estados de cheques propios: *emitido*, *debitado*, *rechazado*.
- **FR-003**: El sistema DEBE crear automáticamente una "Cartera de cheques" por moneda
  al usar la funcionalidad, y una cuenta "Cheques propios a debitar" por cada cuenta
  bancaria que emita cheques.
- **FR-004**: Cobros y pagos DEBEN aceptar como medio uno o más cheques, combinables
  con otros medios.
- **FR-005**: Depósito, cobro por ventanilla y débito de cheques propios DEBEN
  registrarse como transferencias entre cuentas propias.
- **FR-006**: El rechazo DEBE revertir, de forma atómica, los efectos según el estado
  previo del cheque (ver Historia 6).
- **FR-007**: El sistema DEBE ofrecer vistas de cartera y de cheques propios emitidos,
  por fecha de pago, con totales por moneda.
- **FR-008**: La cartera y los cheques propios solo son visibles para Administradores.
  El Operador solo puede recibir cheques como cobro.

### Entidades clave

- **Cheque (Check)**: tipo de origen (tercero/propio), datos de FR-001, estado, cuenta
  bancaria emisora (propios), cliente de origen (terceros), proveedor endosatario.
- **Historial del cheque (CheckEvent)**: fecha, transición de estado, operación
  asociada, usuario.

## Criterios de éxito

- **SC-001**: El dueño sabe en menos de 10 segundos qué cheques puede depositar esta
  semana y cuánto se le va a debitar.
- **SC-002**: Ante un rechazo, las cuentas corrientes y la caja quedan correctas en una
  sola operación, sin asientos manuales.

## Supuestos

- No hay integración con bancos para emitir o consultar e-cheq.
- No se gestiona la chequera (talonarios ni numeración automática de cheques propios).
- No se calculan intereses ni descuentos por negociar cheques (venta a financieras).
