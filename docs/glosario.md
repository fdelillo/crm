# Glosario

Términos del negocio (usados en specs e interfaz) y su nombre en el código.

| Término (ES) | Código (EN) | Definición |
|--------------|-------------|------------|
| Empresa | `Tenant` | Organización que usa el sistema. Sus datos están aislados de otras empresas. |
| Usuario | `User` | Persona que inicia sesión. Pertenece a una empresa con un rol. |
| Rol | `Role` | `admin` (Administrador) u `operator` (Operador). |
| Invitación | `Invitation` | Enlace de un solo uso, válido 7 días, para que un usuario invitado defina su contraseña. |
| Sesión | `Session` | Acceso iniciado desde un dispositivo; vence tras 24 h sin uso o 7 días, y se cierra al salir, al desactivar el usuario o al cambiar la contraseña. |
| Token de un solo uso | `UserToken` | Secreto enviado por email para verificar el email, restablecer la contraseña o aceptar una invitación. |
| Permiso | `Permission` | Capacidad concreta de la matriz de permisos de la spec 001 (FR-007), p. ej. `settings.manage`. |
| Plantilla de rubro | `IndustryTemplate` | Configuración inicial precargada (etapas, categorías, nombres, catálogo). |
| Tercero | `Party` | Persona o empresa con la que se opera. Puede tener uno o más roles. |
| Cliente | `Customer` (rol de `Party`) | Tercero al que se le hacen proyectos. |
| Proveedor | `Supplier` (rol de `Party`) | Tercero al que se le compra. |
| Empleado | `Employee` (rol de `Party`) | Tercero que trabaja para la empresa. |
| Socio | `Partner` (rol de `Party`) | Dueño de la empresa que aporta o retira dinero. |
| Proyecto | `Project` | Trabajo para un cliente (ej. "Aberturas casa Pérez"). El nombre es configurable ("Obra", "Trabajo"). |
| Etapa | `Stage` | Estado de un proyecto dentro del pipeline configurable de la empresa. |
| Presupuesto | `Quote` | Propuesta económica de un proyecto. Un proyecto puede tener varias versiones. |
| Ítem de presupuesto | `QuoteItem` | Línea del presupuesto: descripción, medidas, cantidad y precio. |
| Catálogo | `CatalogItem` | Producto o servicio reutilizable con precio de referencia. |
| Categoría | `Category` | Clasificación de un ingreso o egreso (ej. "Vidrios", "Flete", "Alquiler"). |
| Cuenta de dinero | `MoneyAccount` | Lugar donde está el dinero: caja efectivo, banco, billetera virtual. Tiene una sola moneda. |
| Movimiento de caja | `CashMovement` | Entrada o salida de dinero de una cuenta de dinero. |
| Transferencia | `Transfer` | Paso de dinero entre dos cuentas de dinero propias, incluso de distinta moneda. |
| Cuenta corriente | `PartyLedger` | Registro de lo que un tercero nos debe o le debemos, por rol y moneda. |
| Movimiento de cuenta corriente | `LedgerEntry` | Registro que modifica el saldo de una cuenta corriente. |
| Cobro | `CustomerReceipt` | Dinero que recibimos de un cliente. |
| Compra | `Purchase` | Comprobante de un proveedor que genera deuda a pagar. |
| Pago | `Payment` | Dinero que entregamos a un proveedor o empleado. |
| Cargo a empleado | `EmployeeCharge` | Monto devengado a favor del empleado (sueldo, jornales, horas). |
| Aporte | `Contribution` | Dinero que un socio pone en la empresa. |
| Retiro | `Withdrawal` | Dinero que un socio saca de la empresa. |
| Gasto directo | `DirectExpense` | Egreso sin tercero con cuenta corriente (ej. combustible, impuestos). |
| Ajuste de proyecto | `ProjectAdjustment` | Adicional o bonificación sobre el monto aprobado de un proyecto. |
| Imputación de costo | `CostAllocation` | Asignación total o parcial de un costo a uno o más proyectos. |
| Importe equivalente | `CounterAmount` | En una operación entre dos monedas, el importe en la otra moneda informado por el usuario (ej. los USD que cancela un cobro en ARS). |
| Equivalente de referencia | `ReferenceAmount` | Monto del proyecto expresado en la otra moneda al aprobarlo. Solo informativo. |
| Tipo de cambio implícito | — | Cociente entre los dos importes de una operación. Se muestra como ayuda y **no se guarda**. |
| Moneda base | `BaseCurrency` | Moneda principal de la empresa, usada para reportes consolidados. |
| Cheque | `Check` | Cheque físico o e-cheq, de terceros (recibido) o propio (emitido). |
| Cartera de cheques | `CheckPortfolio` (tipo de `MoneyAccount`) | Cheques de terceros en poder de la empresa. |
| Cheques propios a debitar | `IssuedChecksAccount` (tipo de `MoneyAccount`) | Cheques emitidos que el banco todavía no debitó. |
| Endoso | `Endorsement` | Entrega de un cheque de terceros a un proveedor como pago. |
| Rechazo | `CheckBounce` | Cheque no pagado por el banco. Revierte sus efectos con una operación nueva (no es una anulación). |
| Plan de cobros | `PaymentSchedule` | Cuotas acordadas con el cliente (fecha e importe). Opcional; no cambia la deuda. |
| Cuota | `PaymentScheduleItem` | Una línea del plan de cobros. |
| Proyecto en curso | `OngoingProject` (alta de `Project`) | Proyecto cargado al migrar, con monto acordado y cobrado previo, sin presupuesto. |
| Anulación | `Void` | Invalidación de un movimiento, con motivo. Revierte sus efectos sin borrarlo. |
| Extracto bancario | `BankStatement` | Archivo CSV/Excel con movimientos exportado del banco. |
| Conciliación | `Reconciliation` | Vinculación de una línea de extracto con un movimiento del sistema. |
| Adjunto | `Attachment` | Archivo (foto, plano, PDF) asociado a un proyecto. |
| Nota | `Note` | Comentario en la bitácora de un proyecto. |
| Auditoría | `AuditLog` | Registro de quién hizo qué y cuándo. |
| Mensaje saliente | `OutboxMessage` | Email pendiente de envío, guardado junto con la operación que lo originó. |

## Convención de signos en cuentas corrientes

- **Saldo positivo**: el tercero **nos debe** (deudor). Ej. cliente con saldo pendiente.
- **Saldo negativo**: **le debemos** al tercero (acreedor). Ej. proveedor con compras impagas.

En la interfaz nunca se muestran signos: se usan las leyendas "Te debe" y "Le debés".
