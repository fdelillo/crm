# ADR-029: Representación del dinero en la base y en la API: centavos `bigint` + moneda, objeto `Money`, tope 2^53 − 1

**Status**: Proposed (pendiente de aprobación del usuario junto con el plan de 002)
**Fecha**: 2026-10-23
**Origen**: spec 002 (primer importe del sistema: el precio de referencia del catálogo, plan DD-9);
rige para las specs 004 a 010

## Contexto

La constitución (principio IV) exige guardar importes como **enteros en la unidad mínima** con su
**moneda** ISO 4217, prohíbe el punto flotante y prohíbe sumar monedas distintas. ADR-023
(*Proposed*) fija cómo los muestra y carga el frontend: el contrato manda `amount_cents` entero y la
moneda; el cliente valida `Number.isSafeInteger` y formatea por manipulación de texto.

Falta decidir la forma **del lado del servidor y del contrato**, y 002 es la primera spec con un
importe (el precio de referencia de un ítem de catálogo, opcional). Si cada spec elige su forma,
005 a 010 van a divergir en nombres de columnas, en cómo se representa "sin importe" y en el
máximo admitido.

## Decisión

1. **Base de datos**: un importe es un par de columnas `<nombre>_cents bigint` y
   `<nombre>_currency text`, con:
   - `CHECK (<nombre>_cents BETWEEN 0 AND 9007199254740991)` cuando el importe no tiene signo (el
     sentido lo da la operación, como pide 006). Un importe con signo (p. ej. un saldo derivado)
     define su propio rango explícito en su spec;
   - `CHECK (<nombre>_currency IN ('ARS','USD'))` (monedas del MVP; agregar una es ampliar el
     `CHECK` y el enum del contrato);
   - si el importe es opcional, `CHECK ((<nombre>_cents IS NULL) = (<nombre>_currency IS NULL))`:
     ambos o ninguno.
   Nunca `numeric`, `money`, `real` ni `double precision` para importes.
2. **Go**: `int64` para los centavos y un `string` validado para la moneda. Ninguna conversión pasa
   por `float64`; ningún cálculo mezcla monedas. Cada módulo puede declarar su tipo de valor
   (`settings.Money{AmountCents int64; Currency string}`); si dos módulos llegan a necesitar
   operaciones comunes (sumas por moneda, redondeo), se mueve a un paquete de plataforma con un ADR
   o una nota en este.
3. **API**: un objeto compartido `Money` en el contrato de 001 (componentes compartidos, DD-17 de
   001): `{ "amount_cents": integer (int64, 0..2^53 − 1), "currency": Currency }`, con
   `additionalProperties: false`. "Sin importe" es `null` en el campo que lo contiene, nunca
   `amount_cents: 0`. El servidor rechaza un `amount_cents` con decimales o fuera de rango (`400`
   si no decodifica como entero, `422` con `invalid_value` si está fuera de rango).
4. **Tope 2^53 − 1** (9.007.199.254.740.991 centavos): es el mayor entero que JavaScript representa
   exacto, así el cliente nunca recibe un importe que no puede manejar (ADR-023). La base lo exige
   con `CHECK` aunque `bigint` admita más.
5. **Operaciones entre monedas** (006, 007): se guardan **los dos importes** informados por el
   usuario, cada uno en su par de columnas, y nunca un tipo de cambio (constitución IV). Este ADR
   solo fija la forma de cada importe.

## Fundamento

- Enteros en la unidad mínima son exactos por construcción; la constitución lo exige.
- Un par de columnas con `CHECK` deja la regla en la base, visible y verificable por los tests de
  catálogo, sin tipos compuestos de PostgreSQL que sqlc y pgx mapean peor.
- Un único `Money` en el contrato da un solo tipo generado en el cliente y un solo lugar donde
  cambiar las monedas.
- Representar "sin importe" con `null` evita un 0 con significado oculto (un 0 puede ser un precio
  bonificado).
- El tope de 2^53 − 1 cubre de sobra a una pyme (≈ 90 billones de pesos) y elimina una clase entera
  de errores silenciosos en el cliente.

## Alternativas consideradas

- **`numeric(18,2)`**: decimal exacto en la base, pero la constitución pide la unidad mínima, y en
  Go invita a convertir a `float64` o a sumar una librería decimal.
- **Tipo compuesto de PostgreSQL (`CREATE TYPE money_amount AS (cents bigint, currency text)`)**: un
  solo campo, pero sqlc/pgx lo soportan con más fricción y los `CHECK` sobre el compuesto son
  menos legibles.
- **Importe como string decimal en la API (`"1234.56"`)**: evita el límite de JavaScript, pero el
  cliente tendría que parsear decimales (lo que ADR-023 quiere evitar) y cada spec elegiría un
  formato.
- **Monto en la moneda base de la empresa, sin columna de moneda**: la empresa opera en ARS y en USD
  por producto y por proyecto; la moneda base es solo para consolidar (009).
- **Sin tope explícito**: `bigint` admitiría importes que el cliente no representa exactos; el
  error aparecería en la pantalla, no en la carga.

## Consecuencias

- Ganás: una forma única de importe en todas las specs; reglas en la base; un tipo compartido en el
  contrato y en el cliente.
- Aceptás: dos columnas por importe; agregar una moneda toca `CHECK` de varias tablas y el enum
  `Currency`; un importe con signo necesita su propio esquema en el contrato (no es `Money`).
- El primer cambio del contrato de 001 que lo aplica es su v0.6.0 (T-002-B202 del plan de 002).
