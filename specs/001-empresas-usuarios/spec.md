# Spec: Empresas, usuarios y roles

**Rama**: `001-empresas-usuarios`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: El sistema es SaaS multiempresa. Cada empresa se registra, invita a su
equipo y controla qué ve cada usuario (Administrador / Operador). El acceso es con
email y contraseña.

## Clarificaciones

### Sesión 2026-09-27

- P: ¿Qué puede hacer el Operador además de clientes, proyectos y presupuestos? → R: Registrar cobros de clientes, ver saldos de clientes y proyectos, y ver los datos de contacto de proveedores (sin saldos). No carga gastos ni compras.
- P: ¿Un usuario puede pertenecer a varias empresas? → R: No, a una sola en el MVP.
- P: ¿La anulación tiene límite de antigüedad? → R: No. Solo un Administrador anula, siempre con motivo y auditoría; no hay fecha de cierre.

## Escenarios de usuario y pruebas

### Historia 1: Registrar una empresa (Prioridad: P1)

El dueño de una carpintería se registra con su email, crea la empresa, elige la
plantilla de rubro y queda como Administrador.

**Por qué esta prioridad**: sin empresa ni usuario no se puede usar nada.

**Prueba independiente**: registrarse y ver el panel inicial con la configuración de
la plantilla elegida.

**Escenarios de aceptación**:

1. **Dado** un visitante, **cuando** completa nombre, email, contraseña, nombre de la
   empresa, moneda base y plantilla de rubro, **entonces** se crea la empresa, el
   usuario queda como Administrador y se precarga la configuración de la plantilla.
2. **Dado** un email ya registrado, **cuando** alguien intenta registrarse con él,
   **entonces** el sistema lo rechaza sin revelar datos de la cuenta existente.
3. **Dado** un registro nuevo, **cuando** se completa, **entonces** se envía un email
   para verificar la dirección.

### Historia 2: Iniciar y cerrar sesión (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un usuario activo, **cuando** ingresa email y contraseña correctos,
   **entonces** accede a su empresa.
2. **Dado** 5 intentos fallidos seguidos, **cuando** se intenta de nuevo, **entonces**
   el acceso se bloquea temporalmente (15 minutos).
3. **Dado** un usuario que olvidó su contraseña, **cuando** la pide, **entonces**
   recibe un enlace de un solo uso válido por 1 hora.

### Historia 3: Invitar operadores (Prioridad: P2)

El Administrador invita a un empleado administrativo como Operador.

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** invita un email con rol Operador, **entonces**
   se envía una invitación válida por 7 días.
2. **Dado** un invitado, **cuando** acepta y define su contraseña, **entonces** accede
   con rol Operador.
3. **Dado** un Administrador, **cuando** desactiva a un usuario, **entonces** este no
   puede volver a ingresar y sus sesiones abiertas se cierran.
4. **Dado** que queda un único Administrador activo, **cuando** se intenta desactivarlo
   o bajarlo a Operador, **entonces** el sistema lo impide.

### Historia 4: Datos de la empresa (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** edita razón social, CUIT, dirección, teléfono,
   email y logo, **entonces** esos datos se usan en los PDF de presupuestos.

### Casos borde

- Un usuario que intenta acceder por URL a un recurso de otra empresa recibe "no
  encontrado", no "prohibido", para no confirmar que existe.
- Un Operador que intenta acceder a una pantalla restringida recibe un error de permiso.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE permitir el registro autónomo de una empresa con su
  primer usuario Administrador.
- **FR-002**: El sistema DEBE ofrecer plantillas de rubro al registrarse: como mínimo
  "Carpintería de aluminio" y "Genérico".
- **FR-003**: El sistema DEBE autenticar con email y contraseña, guardando las
  contraseñas con un hash robusto (argon2id o bcrypt).
- **FR-004**: El sistema DEBE permitir recuperar la contraseña por email.
- **FR-005**: El sistema DEBE permitir al Administrador invitar, desactivar y cambiar
  el rol de usuarios.
- **FR-006**: El sistema DEBE aislar los datos de cada empresa (Constitución, principio III).
- **FR-007**: El sistema DEBE aplicar la siguiente matriz de permisos:

| Funcionalidad | Administrador | Operador |
|---------------|:-------------:|:--------:|
| Clientes (alta, edición, consulta) | ✔ | ✔ |
| Proyectos, notas, adjuntos, cambio de etapa | ✔ | ✔ |
| Presupuestos (crear, PDF, enviar, aprobar) | ✔ | ✔ |
| Registrar cobros de clientes (incluso con cheques) | ✔ | ✔ |
| Ver saldo de un cliente o proyecto | ✔ | ✔ |
| Ver costos y margen de un proyecto | ✔ | ✗ |
| Proveedores: datos de contacto (sin saldos ni movimientos) | ✔ | ✔ |
| Proveedores (cuenta corriente), empleados y socios | ✔ | ✗ |
| Cuentas de dinero (saldos y movimientos) | ✔ | ✗ |
| Compras, pagos, cargos, aportes, retiros, gastos | ✔ | ✗ |
| Cartera de cheques y cheques propios | ✔ | ✗ |
| Importación de extractos | ✔ | ✗ |
| Reportes | ✔ | ✗ |
| Configuración y usuarios | ✔ | ✗ |
| Anular movimientos (sin límite de antigüedad) | ✔ | ✗ |

- **FR-008**: El sistema DEBE registrar en la auditoría los inicios de sesión, las
  invitaciones y los cambios de rol.

### Entidades clave

- **Empresa (Tenant)**: nombre, razón social, CUIT, dirección, contacto, logo, moneda
  base, zona horaria, plantilla de origen.
- **Usuario (User)**: nombre, email, estado (invitado, activo, desactivado), rol.

## Criterios de éxito

- **SC-001**: Un dueño registra su empresa y llega al panel en menos de 3 minutos.
- **SC-002**: Los tests automatizados demuestran 0 accesos cruzados entre empresas en
  todos los endpoints.

## Supuestos

- En el MVP un usuario pertenece a **una sola empresa**. Pertenecer a varias queda para
  una versión futura.
- No hay autenticación de dos factores en el MVP.
- El envío de emails usa un proveedor transaccional externo (se define en el plan).
