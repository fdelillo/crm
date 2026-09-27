# Spec: Reportes

**Rama**: `009-reportes`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: El dueño necesita ver de un vistazo cuánto dinero tiene, quién le debe, a
quién le debe y cómo se movió la caja en el tiempo, incluyendo lo que se espera cobrar
y pagar.

## Clarificaciones

### Sesión 2026-09-27

- P: ¿Qué usa la proyección de cobros? → R: El plan de cobros del proyecto si existe; si no, la fecha estimada de entrega.

## Escenarios de usuario y pruebas

### Historia 1: Saldos de caja por cuenta (Prioridad: P1)

**Por qué esta prioridad**: es la pregunta más frecuente del dueño.

**Prueba independiente**: con cuentas en ARS y USD, ver saldos y totales por moneda.

**Escenarios de aceptación**:

1. **Dado** cuentas de dinero con movimientos, **cuando** se abre el reporte, **entonces**
   se ve el saldo de cada cuenta y el total por moneda, a hoy o a una fecha elegida.
2. **Dado** un tipo de cambio ingresado por el usuario, **entonces** se muestra además
   un total consolidado en la moneda base. Ese tipo de cambio se usa solo para la
   consulta y no se guarda.

### Historia 2: Deudores y acreedores (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** terceros con saldo, **cuando** se abre el reporte, **entonces** se ven dos
   listas: **"Te deben"** (clientes, empleados con adelantos, etc.) y **"Les debés"**
   (proveedores, empleados, socios), por moneda y ordenadas por importe.
2. **Dado** el reporte, **cuando** se filtra por rol (cliente, proveedor, empleado,
   socio), **entonces** se muestran solo esos terceros.
3. **Dado** clientes deudores, **entonces** se muestra el detalle por proyecto, la
   antigüedad del saldo (0–30, 31–60, 61–90, más de 90 días) y las cuotas vencidas
   del plan de cobros.
4. **Dado** compras con vencimiento, **entonces** se destacan las vencidas y las que
   vencen en los próximos 7 días.
5. **Dado** un deudor, **cuando** se toca "Recordar por WhatsApp", **entonces** se abre
   WhatsApp con un mensaje con su saldo.

### Historia 3: Flujo de caja por período (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un rango de fechas, **cuando** se abre el reporte, **entonces** se ven
   ingresos y egresos reales por mes y por categoría, con el neto de cada mes y el
   saldo acumulado, por moneda.
2. **Dado** el reporte, **entonces** las transferencias entre cuentas propias y los
   movimientos anulados no se incluyen.
3. **Dado** la opción "Incluir proyección", **entonces** se agregan los meses futuros con:
   - **cobros esperados**: cuotas pendientes del plan de cobros de cada proyecto, en
     su fecha; si el proyecto no tiene plan, su saldo pendiente en la fecha estimada
     de entrega o, si no la tiene, en el mes actual;
   - **pagos esperados**: compras pendientes, en su fecha de vencimiento o, si no la
     tiene, en el mes actual;
   - **cheques**, en una sección informativa aparte: los de terceros en cartera por
     fecha de pago y los propios emitidos por fecha de débito (ver
     [010](../010-cheques/spec.md)). No suman a ingresos ni egresos, porque ya se
     contaron al recibirlos o emitirlos.
4. **Dado** el reporte, **cuando** se toca una celda (mes × categoría), **entonces** se
   ven los movimientos que la componen.
5. **Dado** cualquier reporte, **entonces** se puede exportar a Excel/CSV.

### Casos borde

- Los reportes respetan la zona horaria de la empresa al agrupar por mes.
- Aportes y retiros de socios se muestran en una sección separada de los ingresos y
  egresos operativos, para no distorsionar el resultado del negocio.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE ofrecer los reportes: saldos por cuenta, deudores y
  acreedores, y flujo de caja por período con proyección opcional.
- **FR-002**: Todos los reportes DEBEN separar por moneda y permitir un consolidado en
  la moneda base con tipo de cambio ingresado por el usuario al consultar (no se guarda).
- **FR-003**: Todos los reportes DEBEN permitir ver el detalle de los movimientos que
  componen cada cifra.
- **FR-004**: Todos los reportes DEBEN poder exportarse a Excel/CSV.
- **FR-005**: Los reportes son exclusivos para Administradores.
- **FR-006**: El panel de inicio del Administrador DEBE mostrar un resumen: saldo total
  por moneda, total "Te deben", total "Les debés" y neto del mes en curso.

## Criterios de éxito

- **SC-001**: Cada reporte carga en menos de 2 segundos con 3 años de datos de una
  empresa típica (~20.000 movimientos).
- **SC-002**: Las cifras de los reportes coinciden exactamente con la suma de los
  movimientos (verificado por tests).

## Supuestos

- No hay reporte consolidado de rentabilidad entre proyectos en el MVP; cada proyecto
  muestra su margen en su ficha.
- No hay gráficos avanzados en el MVP: tablas con totales y, como máximo, un gráfico de
  barras de ingresos/egresos mensuales.
