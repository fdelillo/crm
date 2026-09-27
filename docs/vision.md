# Visión del producto

## Problema

Las pequeñas empresas que trabajan por proyecto (carpinterías de aluminio, herrerías,
constructoras chicas, instaladores, talleres) suelen llevar clientes, presupuestos y
dinero en planillas, cuadernos y WhatsApp. Por eso:

- no saben con certeza cuánto les debe cada cliente ni cuánto deben a cada proveedor;
- no saben si un proyecto dejó ganancia;
- mezclan el dinero de la empresa con el de los socios;
- pierden tiempo armando presupuestos a mano.

Los CRM y sistemas de gestión del mercado son demasiado complejos, están orientados a
la venta (embudos, marketing) o exigen conocimientos contables.

## Propuesta

Un sistema web **sencillo**, usable desde el celular, que responde tres preguntas:

1. **¿Qué trabajos tengo y en qué estado están?** Clientes, proyectos y presupuestos.
2. **¿Quién me debe y a quién le debo?** Cuentas corrientes de clientes, proveedores,
   empleados y socios.
3. **¿Cuánta plata tengo y cómo se mueve?** Cuentas de dinero y flujo de caja.

## Primer cliente: carpintería de aluminio

Fabrica e instala aberturas de aluminio (ventanas, puertas), mamparas de baño, etc.
Su circuito típico:

1. Un cliente consulta y se toman medidas.
2. Se arma un presupuesto con ítems (ej. "Ventana corrediza 1,50 × 1,10 m").
   Muchas veces hay alternativas (con o sin DVH, distintas líneas de perfil).
3. El cliente aprueba y deja una seña.
4. Se compran materiales (perfiles, vidrios, accesorios) a proveedores.
5. Se fabrica en el taller y se instala en obra.
6. Se cobra el saldo y se cierra el proyecto.

Paralelamente se paga a empleados (jornales, adelantos) y los socios aportan o retiran
dinero. Se opera en pesos y en dólares.

## Usuarios

| Perfil | Quién es | Qué necesita |
|--------|----------|--------------|
| **Administrador** | Dueño o socio de la empresa | Ver todo: caja, deudas, socios, reportes. Configurar el sistema. |
| **Operador** | Empleado administrativo o vendedor | Cargar clientes, proyectos y presupuestos; registrar cobros. No ve finanzas sensibles. |

## Alcance del MVP

| Módulo | Spec |
|--------|------|
| Empresas, usuarios y roles | [001](../specs/001-empresas-usuarios/spec.md) |
| Configuración por empresa (etapas, nombres, categorías, catálogo) | [002](../specs/002-configuracion/spec.md) |
| Terceros: clientes, proveedores, empleados, socios | [003](../specs/003-terceros/spec.md) |
| Proyectos: etapas, notas, adjuntos, costos | [004](../specs/004-proyectos/spec.md) |
| Presupuestos: ítems, versiones, IVA, PDF, WhatsApp | [005](../specs/005-presupuestos/spec.md) |
| Cuentas de dinero y movimientos de caja | [006](../specs/006-cuentas-dinero/spec.md) |
| Cuentas corrientes: cobros, compras, pagos, cargos, aportes y retiros | [007](../specs/007-cuentas-corrientes/spec.md) |
| Importación de extractos bancarios (CSV/Excel) | [008](../specs/008-importacion-extractos/spec.md) |
| Reportes: saldos, deudores/acreedores, flujo de caja | [009](../specs/009-reportes/spec.md) |
| Cheques: cartera de terceros, cheques propios, endoso y rechazo | [010](../specs/010-cheques/spec.md) |

### Fuera del MVP

- Facturación electrónica (AFIP/ARCA). El IVA en presupuestos es solo informativo.
- Stock e inventario de materiales.
- Liquidación de sueldos. Los empleados solo tienen cuenta corriente.
- Conexión automática por API con bancos o billeteras.
- Reporte consolidado de rentabilidad entre proyectos. Cada proyecto muestra su propio margen.
- Contabilidad formal (plan de cuentas, partida doble, balances).
- Funcionamiento offline.
- Roles configurables (solo Administrador y Operador).
- Cotización automática del dólar desde fuentes externas.
- Cobro de la suscripción SaaS a las empresas.
- Aceptación online de presupuestos por el cliente (la aprobación la registra un usuario).
- Usuarios que pertenecen a más de una empresa.
- Cierre de períodos (bloqueo de cargas o anulaciones por fecha).
- Detalle de ítems en compras a proveedores (se cargan por importe total).
- Integración con bancos para emitir o consultar e-cheq.

## Orden de construcción sugerido

Cada paso deja algo usable:

1. **001 → 002 → 003**: la empresa se registra, se configura y carga su agenda de terceros.
2. **004 → 005**: gestión comercial (proyectos y presupuestos con PDF). Ya aporta valor sin finanzas.
3. **006 → 007 → 010**: núcleo financiero, incluidos los cheques.
4. **009**: reportes.
5. **008**: importación de extractos.

## Métricas de éxito del producto

- La carpintería deja de usar planillas para cuentas corrientes dentro del primer mes.
- Un presupuesto se arma y envía por WhatsApp en menos de 5 minutos.
- Los saldos del sistema coinciden con el dinero real al hacer un arqueo.
- Una segunda empresa de otro rubro opera **sin cambios de código**, solo con configuración.
