# ADR-023: Idioma, formato de fechas y dinero en el frontend (es-AR, `Intl`, centavos sin punto flotante)

**Status**: Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`ui.md` §12, §19); rige para todas las specs (en especial 005 a 010)

## Contexto

La interfaz es solo en español de Argentina. Las fechas se guardan en UTC y se muestran en la zona
horaria de la empresa (constitución, convenciones; `tenant.timezone`). A partir de 005 la UI
muestra y carga importes: la constitución (principio IV) prohíbe el punto flotante para dinero,
obliga a mostrar siempre la moneda y prohíbe sumar monedas distintas. El contrato manda los
importes como enteros en centavos (`int64`) + moneda ISO 4217. En JavaScript, `number` representa
enteros exactos solo hasta 2^53 − 1 (≈ 90 billones de pesos en centavos) y una división por 100
produce un `float`.

## Decisión

- **Idioma**: textos en español rioplatense escritos directamente en los componentes; sin
  librería de i18n. Los mensajes por `code` de error están centralizados en
  `src/api/errorMessages.ts` (`Record<ErrorCode, …>`). `<html lang="es-AR">`.
- **Formato con `Intl`**, locale `es-AR`, en funciones de `src/lib/format.ts`; ningún componente
  formatea por su cuenta.
- **Fechas y horas**: el servidor manda ISO-8601 en UTC; se muestran con
  `Intl.DateTimeFormat('es-AR', { timeZone: session.tenant.timezone, … })`. En pantallas sin
  sesión (p. ej. vista previa de una invitación) se usa la zona del navegador. Las **fechas sin
  hora** (p. ej. vencimiento de un cheque, `YYYY-MM-DD`, specs futuras) se tratan como texto de
  calendario: nunca `new Date('YYYY-MM-DD')` (se interpreta en UTC y puede correr un día).
- **Dinero** (se implementa en la primera spec que muestre importes):
  - En el cliente, un importe es `{ amount_cents: number; currency: Currency }` tal como llega del
    contrato; se valida `Number.isSafeInteger`.
  - **Formateo sin aritmética de punto flotante**: los centavos se convierten a un string decimal
    exacto por manipulación de texto (`123456` → `"1234.56"`) y se pasa ese string a
    `Intl.NumberFormat('es-AR', { style: 'currency', currency }).format(…)`, que interpreta strings
    como decimales exactos (Chrome ≥ 106, Safari ≥ 15.4, Firefox ≥ 116; cubiertos por los mínimos
    de ADR-016).
  - **Carga**: el texto que escribe el usuario (`1.234,56`) se convierte a centavos por
    manipulación de texto, nunca con `parseFloat`/`Number()` sobre decimales.
  - El cliente **no calcula saldos** ni suma importes para mostrarlos: los saldos los deriva el
    servidor (principio IV). Si alguna vez suma (p. ej. el total de los ítems de un presupuesto en
    edición), suma **enteros** de centavos de **una sola moneda**.
  - El tipo de cambio implícito, si se muestra como ayuda, se calcula solo para mostrar y nunca se
    envía ni se guarda.
- **Firmas** (contrato para las specs siguientes):
  `formatMoney(amountCents: number, currency: Currency): string`,
  `parseMoneyInput(text: string): { ok: true; amountCents: number } | { ok: false; reason: 'format' | 'too_large' }`,
  `formatDateTime(iso: string, timeZone: string): string`, `formatDate(isoDate: string): string`.

## Fundamento

- Un solo idioma: una librería de i18n agrega claves, archivos y un paso de indirección sin
  beneficio. Si algún día hay otro idioma, se reemplaza este ADR.
- `Intl` es nativo, conoce las convenciones de es-AR (punto de miles, coma decimal, `$`) y las
  zonas IANA.
- Pasar strings decimales a `Intl.NumberFormat` evita por construcción cualquier error de
  redondeo, sin librerías de dinero.
- Centralizar el formato hace que la regla de "nunca float" se revise en un solo archivo.

## Alternativas consideradas

- **react-intl / i18next**: estándar para apps multilenguaje; innecesario con un solo idioma.
- **`amount_cents / 100` + `Intl.NumberFormat`**: funciona en la práctica para montos chicos, pero
  es aritmética de punto flotante sobre dinero (prohibida por la constitución) y pierde exactitud
  en montos muy grandes.
- **Librerías de dinero (dinero.js, big.js)**: exactas, pero una dependencia para algo que
  `Intl` + manipulación de strings resuelve.
- **`bigint` para centavos**: exacto sin límite, pero `JSON.parse` entrega `number` y habría que
  interceptar el parseo; los montos de una pyme están muy por debajo de 2^53.
- **Mostrar fechas en la zona del navegador**: más simple, pero un usuario de viaje vería horas
  distintas a las de su empresa; la constitución pide la zona de la empresa.

## Consecuencias

- Ganás: textos directos y legibles, formato correcto de fechas y dinero sin dependencias, la regla
  de "nunca float" aplicada en un único lugar.
- Aceptás: cambiar un texto es buscarlo en el componente (no hay catálogo, salvo los errores); los
  importes mayores a 2^53 − 1 centavos se rechazan en el cliente (no son realistas para el
  público objetivo); las funciones de dinero se implementan y prueban en la primera spec que las
  use, no en 001.
