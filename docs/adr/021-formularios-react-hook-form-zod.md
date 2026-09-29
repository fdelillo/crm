# ADR-021: Formularios con React Hook Form y validación con Zod atada a los tipos del contrato

**Status**: Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`ui.md` §16)

## Contexto

001 tiene siete formularios (registro con 6 campos, datos de la empresa con 7, login, reset,
invitación, aceptar invitación, olvidé mi contraseña) y las specs siguientes tienen formularios
más largos (presupuestos, cobros). La validación del cliente es UX; la del servidor es la
autoridad y responde `422` con `errors[{field, code}]` (ADR-009). Las reglas replicadas en el
cliente (longitudes, formatos, CUIT) no pueden divergir del contrato.

## Decisión

- **React Hook Form** para el estado de cada formulario, con los componentes `Field` de shadcn
  (ADR-016) para etiqueta, ayuda y error.
- **Zod** (v4) con `@hookform/resolvers` (≥ 5.1, que soporta Zod 4) para las reglas del cliente. Cada
  esquema se declara contra el tipo del request generado del contrato (p. ej. `satisfies
  z.ZodType<SignupRequest>` o equivalente): si el contrato cambia un campo, no compila.
- **Nombres de campo = propiedades del contrato** (`snake_case`), para aplicar los `FieldError`
  del `422` directo con `setError` (`applyServerFieldErrors`).
- **Cuándo se valida**: al enviar; después del primer intento, al cambiar cada campo
  (`mode: 'onSubmit'`, `reValidateMode: 'onChange'`). Foco en el primer campo con error.
- Solo se replican reglas que están en el contrato o en un `DD` del plan (DD-6, DD-16), cada una con
  su test. Ninguna regla vive solo en el cliente.
- `PATCH` parciales envían solo los campos modificados (`dirtyFields`).

## Fundamento

- Desde tres campos con validación, una librería ahorra más de lo que cuesta: registro de campos,
  errores por campo, foco, estado de envío y campos modificados vienen resueltos.
- React Hook Form usa inputs no controlados: pocos re-renders al tipear en celulares de gama media.
- Zod expresa las reglas en un lugar legible y se integra con los tipos de TypeScript; atarlo al
  tipo del contrato evita la deriva.
- Nombres iguales al contrato eliminan una tabla de traducción de campos.

## Alternativas consideradas

- **Formularios nativos controlados con `useState`**: sin dependencias, pero cada formulario
  reimplementa errores, foco y estado de envío.
- **TanStack Form**: muy bien tipado y del mismo ecosistema que Query, pero más nuevo, con API más
  verbosa y menos ejemplos con shadcn.
- **Acciones de formulario de React 19 (`useActionState`)**: nativo, pero pensado para acciones del
  servidor/SSR; en una SPA hay que armar validación por campo y foco a mano.
- **Generar esquemas Zod desde el OpenAPI** (orval, openapi-zod-client): menos duplicación de
  reglas, pero otra herramienta de generación con su propio código; las reglas del cliente son
  pocas y conviene poder leerlas.
- **Valibot / Zod mini**: más livianos; se reevalúan si el presupuesto de JS se excede (RF-7).

## Consecuencias

- Ganás: formularios consistentes, errores del servidor en el campo correcto sin traducción,
  reglas que no divergen del contrato en tipos.
- Aceptás: dos librerías más que aprender; los **valores** de las reglas (p. ej. 10 caracteres de
  contraseña) se copian del contrato y se vigilan con tests, porque el tipo no los transporta.
