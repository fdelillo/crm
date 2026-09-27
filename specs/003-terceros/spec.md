# Spec: Terceros (clientes, proveedores, empleados, socios)

**Rama**: `003-terceros`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: Una agenda única de personas y empresas con las que se opera. Cada
tercero puede cumplir uno o más roles: cliente, proveedor, empleado o socio.

## Clarificaciones

### Sesión 2026-09-27

- P: ¿El Operador ve proveedores? → R: Sí, solo sus datos de contacto, sin saldos ni movimientos.

## Escenarios de usuario y pruebas

### Historia 1: Cargar y buscar clientes (Prioridad: P1)

**Por qué esta prioridad**: es la base de proyectos y presupuestos.

**Prueba independiente**: dar de alta un cliente desde el celular y encontrarlo
después por nombre o teléfono.

**Escenarios de aceptación**:

1. **Dado** un usuario, **cuando** crea un cliente con al menos nombre y un teléfono o
   email, **entonces** queda disponible para asignarle proyectos.
2. **Dado** una lista de clientes, **cuando** busca por nombre, teléfono, email o
   CUIT/DNI, **entonces** ve los resultados mientras escribe.
3. **Dado** un teléfono ya cargado en otro tercero, **cuando** se crea un cliente con
   ese teléfono, **entonces** el sistema avisa del posible duplicado, pero permite continuar.

### Historia 2: Ficha del tercero (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un cliente, **cuando** se abre su ficha, **entonces** se ven sus datos,
   sus proyectos y su saldo por moneda ("Te debe").
2. **Dado** un proveedor, empleado o socio, **cuando** el Administrador abre su ficha,
   **entonces** ve sus datos, su cuenta corriente por moneda y los últimos movimientos.
3. **Dado** un teléfono cargado, **cuando** se toca "WhatsApp", **entonces** se abre
   una conversación con ese número.

### Historia 3: Varios roles para un mismo tercero (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un socio que también cobra como empleado, **cuando** se le asignan ambos
   roles, **entonces** tiene dos cuentas corrientes separadas (socio y empleado).
2. **Dado** un proveedor que también es cliente, **entonces** sus saldos como
   proveedor y como cliente se muestran por separado y no se compensan
   automáticamente.

### Historia 4: Proveedores, empleados y socios (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** da de alta un proveedor, empleado o socio,
   **entonces** se crea con su rol y queda disponible para operaciones de cuenta corriente.
2. **Dado** un Operador, **entonces** ve los datos de contacto de los proveedores,
   pero no su cuenta corriente, y no ve empleados ni socios (ver matriz de permisos en
   [001](../001-empresas-usuarios/spec.md)).

### Casos borde

- Un tercero con movimientos no se puede eliminar; solo **archivar**.
- Al quitar un rol que tiene saldo distinto de cero, el sistema lo impide y muestra
  el saldo.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE gestionar terceros con: nombre o razón social, tipo
  (persona/empresa), CUIT/DNI (opcional), teléfonos, email, dirección, notas.
- **FR-002**: El sistema DEBE permitir asignar a un tercero uno o más roles: cliente,
  proveedor, empleado, socio.
- **FR-003**: El sistema DEBE mantener una cuenta corriente por **tercero, rol y
  moneda** (ver [007](../007-cuentas-corrientes/spec.md)).
- **FR-004**: El sistema DEBE ofrecer búsqueda incremental por nombre, teléfono,
  email y documento.
- **FR-005**: El sistema DEBE advertir posibles duplicados por teléfono, email o documento.
- **FR-006**: El sistema DEBE permitir archivar terceros y ocultarlos de las listas por
  defecto.
- **FR-007**: El sistema DEBE ofrecer enlace directo a WhatsApp y a llamada desde la ficha.
- **FR-008**: El sistema DEBE permitir registrar, para empleados, un dato de referencia
  opcional (ej. jornal o sueldo acordado) como ayuda al cargar cargos.

### Entidades clave

- **Tercero (Party)**: datos de contacto, roles, archivado.
- **Rol de tercero (PartyRole)**: cliente, proveedor, empleado o socio.

## Criterios de éxito

- **SC-001**: Cargar un cliente nuevo desde el celular lleva menos de 30 segundos.

## Supuestos

- Validar el formato del CUIT es una ayuda, no un bloqueo.
- No se importan contactos desde archivos en el MVP.
