# Spec: Configuración por empresa

**Rama**: `002-configuracion`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: Para servir a distintos rubros sin cambiar código, cada empresa puede
configurar las etapas de sus proyectos, los nombres de algunas entidades, sus
categorías de ingresos/egresos y su catálogo de productos/servicios.

## Escenarios de usuario y pruebas

### Historia 1: Configurar etapas de proyecto (Prioridad: P1)

**Por qué esta prioridad**: los proyectos no existen sin etapas.

**Prueba independiente**: crear, renombrar y reordenar etapas, y verlas reflejadas
en el tablero de proyectos.

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** agrega, renombra o reordena etapas,
   **entonces** los proyectos muestran el nuevo pipeline.
2. **Dado** una etapa, **cuando** se la marca como "de cierre" (tipo *ganado* o
   *perdido*), **entonces** los proyectos en esa etapa se consideran cerrados.
3. **Dado** una etapa con proyectos, **cuando** se intenta eliminarla, **entonces** el
   sistema pide moverlos a otra etapa antes de eliminarla.
4. **Dado** cualquier configuración, **entonces** siempre existe al menos una etapa
   inicial y una de cierre *ganado*.

### Historia 2: Personalizar nombres (Prioridad: P3)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** cambia "Proyecto" por "Obra" (singular y
   plural), **entonces** toda la interfaz y los PDF usan "Obra".

### Historia 3: Categorías de ingresos y egresos (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** crea una categoría indicando si es de ingreso
   o egreso, **entonces** queda disponible al registrar movimientos.
2. **Dado** una categoría con movimientos, **cuando** se intenta eliminarla, **entonces**
   solo se puede **archivar** (deja de ofrecerse, pero se conserva en el historial).
3. **Dado** las categorías de sistema (ej. "Cobros de clientes"), **entonces** no se
   pueden eliminar ni archivar; sí renombrar.

### Historia 4: Catálogo de productos/servicios (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** crea un ítem con nombre, descripción, unidad
   (unidad, m², metro lineal, hora, otra), precio de referencia y moneda, **entonces**
   puede usarlo al armar presupuestos.
2. **Dado** un ítem del catálogo, **cuando** se cambia su precio, **entonces** los
   presupuestos ya creados **no** se modifican.
3. **Dado** un ítem que ya no se vende, **cuando** se archiva, **entonces** deja de
   ofrecerse en presupuestos nuevos.

### Historia 5: Cuentas de dinero iniciales y monedas (Prioridad: P1)

Ver [006](../006-cuentas-dinero/spec.md). La plantilla crea por defecto "Caja efectivo ARS".

### Casos borde

- Renombrar una etapa o categoría no altera los datos históricos, que la referencian
  por identificador.
- Una plantilla aplicada solo precarga datos: luego la empresa los modifica libremente.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE permitir a cada empresa definir etapas ordenadas, cada
  una de tipo *abierta*, *ganado* o *perdido*, con un color.
- **FR-002**: El sistema DEBE permitir personalizar los nombres singular y plural de:
  Proyecto, Cliente y Presupuesto.
- **FR-003**: El sistema DEBE permitir crear, renombrar y archivar categorías de
  ingreso y de egreso.
- **FR-004**: El sistema DEBE incluir categorías de sistema no eliminables para las
  operaciones automáticas: Cobros de clientes, Pagos a proveedores, Pagos a empleados,
  Aportes de socios, Retiros de socios.
- **FR-005**: El sistema DEBE permitir gestionar un catálogo de ítems con unidad,
  precio de referencia y moneda.
- **FR-006**: El sistema DEBE definir plantillas de rubro que precargan etapas,
  categorías, nombres y catálogo de ejemplo (ver
  [modelo de dominio](../../docs/modelo-dominio.md#configuración-por-empresa)).
- **FR-007**: El sistema DEBE permitir configurar el texto por defecto de condiciones
  comerciales y la validez (en días) de los presupuestos.

### Entidades clave

- **Etapa (Stage)**: nombre, orden, tipo (abierta/ganado/perdido), color.
- **Categoría (Category)**: nombre, tipo (ingreso/egreso), sistema (sí/no), archivada.
- **Ítem de catálogo (CatalogItem)**: nombre, descripción, unidad, precio de
  referencia, moneda, archivado.
- **Etiquetas (Labels)**: nombres personalizados por entidad.

## Criterios de éxito

- **SC-001**: Una empresa de otro rubro (ej. una herrería) queda operativa solo
  configurando estas opciones, sin cambios de código.

## Supuestos

- Las plantillas de rubro las mantiene el equipo del producto; las empresas no crean
  plantillas propias en el MVP.
- No hay campos personalizados en el MVP.

---

## Propuesta de clarificaciones y de redacción (pendiente de aprobación, 2026-10-23)

> Propuesta del `backend-architect` a partir del plan técnico ([`plan.md`](plan.md) §14.2). **El
> estado de la spec sigue en Borrador**: nada de esta sección rige hasta que el usuario la apruebe.
> El texto de arriba no se modificó. Al aprobarse, las respuestas pasan a una sección
> "Clarificaciones — Sesión 2026-10-23" y los cambios de redacción se aplican al cuerpo; esta
> sección se borra.

### Preguntas que cambian el alcance o el modelo (P-1 a P-5)

**P-1. La Historia 5 (cuentas de dinero iniciales y monedas) depende de 006, que todavía no existe.**

- **A (recomendada)**: sacarla de 002 y moverla a 006. Al implementarse 006, toda empresa nueva
  recibe una cuenta "Caja efectivo" **en su moneda base**, y las empresas existentes la reciben con
  la migración de 006. No hay configuración de monedas: ARS y USD están siempre disponibles y la
  moneda base se fija al registrarse (001).
- B: dejarla en 002 como requisito diferido, sin implementación hasta 006.
- C: crear la tabla de cuentas en 002 solo para sembrar la caja.
- *Por qué A*: 002 no puede crear una cuenta sin el modelo de cuentas de 006 (C adelantaría media
  spec); B deja en 002 una historia que no se puede probar. Además "Caja efectivo ARS" fija sería
  incorrecta para una empresa con moneda base USD.

**P-2. ¿Qué pasa con los proyectos al eliminar una etapa o al cambiar su tipo?** (Los proyectos
llegan en 004.)

- **A (recomendada)**:
  - *Cambiar el tipo* se permite siempre que se respeten las reglas del pipeline (P-6). "Cerrado"
    no se guarda en el proyecto: se deriva del tipo de la etapa en la que está, así que el cambio
    afecta a sus proyectos al instante, sin escribirlos.
  - *Eliminar* una etapa en 002 la saca del pipeline sin vuelta atrás (se conserva internamente
    para el historial). La regla "si tiene proyectos, pedir moverlos a otra etapa" (escenario 1.3)
    se implementa y se verifica en 004, que es donde existen los proyectos.
- B: prohibir cambiar el tipo de una etapa que tenga proyectos.
- C: guardar "cerrado" en cada proyecto y actualizarlo al cambiar el tipo.
- *Por qué A*: el escenario 1.2 ya dice que los proyectos de una etapa de cierre "se consideran
  cerrados": derivarlo evita sincronizar dos datos (C) y no agrega una restricción que la spec no
  pide (B).

**P-3. Alcance de los nombres personalizados** (la UI existe; los PDF llegan en 005).

- **A (recomendada)**: la interfaz los usa desde 002; los PDF, desde 005 (pasa a ser un criterio
  de aceptación de 005). Los emails del sistema (verificación, invitación) no los usan. Cada nombre
  tiene singular, plural **y género gramatical** (masculino o femenino).
- B: igual que A pero sin género.
- C: posponer toda la Historia 2 hasta 005.
- *Por qué A*: sin género la interfaz escribiría "Nuevo Obra" u "Obras abiertos"; los PDF no
  existen todavía, así que no se pueden verificar en 002.

**P-4. ¿Qué datos concretos tiene cada plantilla de rubro?**

- **A (recomendada)**: el contenido de las tablas de abajo ("Contenido propuesto de las
  plantillas"). Las dos plantillas usan los nombres por defecto (Proyecto, Cliente, Presupuesto):
  "Obra" es opcional (modelo de dominio) y el Administrador lo cambia en un minuto. Los ítems de
  catálogo de ejemplo **no traen precio** (ver P-9). La plantilla se aplica **solo al registrarse**:
  no se reaplica ni se cambia de rubro después.
- B: que la plantilla de carpintería use "Obra".
- C: ítems de ejemplo con precios.
- *Por qué A*: hacen falta datos concretos para implementar y probar; un precio de ejemplo queda
  viejo con la inflación y no sabe en qué moneda trabaja cada empresa.

**P-5. ¿Quién puede leer la configuración?**

- **A (recomendada)**: **leer** (etapas, categorías, catálogo, nombres, condiciones) puede cualquier
  usuario de la empresa; **modificar**, solo el Administrador (fila "Configuración y usuarios" de la
  matriz de 001).
- B: solo el Administrador lee y modifica.
- *Por qué A*: el Operador trabaja con proyectos (necesita las etapas) y presupuestos (necesita el
  catálogo y las condiciones), y toda la interfaz muestra los nombres personalizados. Leer la
  configuración no muestra importes ni saldos.

### Defaults a confirmar (P-6 a P-10; se pueden aprobar en bloque)

**P-6. Reglas de las etapas.** Recomendado: la **etapa inicial es la primera** del pipeline y
siempre es *abierta* (los proyectos nuevos empiezan ahí); tiene que haber al menos una de cierre
*ganado*; *perdido* es opcional. Se rechaza cualquier cambio que deje una etapa de cierre primera o
ninguna *ganado*. No puede haber dos etapas con el mismo nombre (sin distinguir mayúsculas).
Alternativa: elegir la etapa inicial con una marca aparte (un concepto más para el usuario).

**P-7. Color de las etapas.** Recomendado: una **paleta cerrada de 8 colores** con nombre (gris,
azul, verde azulado, verde, amarillo, naranja, rojo, violeta); la interfaz decide el tono exacto.
Alternativa: color libre. *Por qué*: habrá un rediseño visual; un color guardado como código
quedaría atado al diseño actual.

**P-8. Categorías.** Recomendado: las categorías **se archivan y se restauran, nunca se eliminan**
(también las que todavía no tienen movimientos); el tipo (ingreso o egreso) no cambia después de
crearla; no puede haber dos activas con el mismo nombre y tipo. Las de sistema son las cinco de
FR-004; las specs siguientes agregan las suyas (p. ej. "Ajuste de arqueo" en 006). Alternativa:
eliminar las que no tienen movimientos (exige saber si los tiene, y los movimientos llegan en 006).

**P-9. Catálogo.** Recomendado: el **precio de referencia es opcional** (si se carga, va con su
moneda; sin precio significa "a cotizar", y 0 es un precio válido); la unidad "otra" exige escribir
su nombre (p. ej. "jornada"); los ítems se archivan y se restauran; puede haber dos ítems con el
mismo nombre. Alternativa: precio obligatorio (las plantillas tendrían que inventar precios).

**P-10. FR-007 no tiene historia.** Recomendado: una **Historia 6 (P2)**, abajo. Condiciones: texto
opcional de hasta 2.000 caracteres; validez de 1 a 365 días; la plantilla define los valores
iniciales; los presupuestos (005) los proponen al crearse.

### Contenido propuesto de las plantillas (P-4)

Comunes a las dos plantillas:

| Elemento | Valor |
|---|---|
| Nombres | Proyecto / Proyectos (masculino) · Cliente / Clientes (masculino) · Presupuesto / Presupuestos (masculino) |
| Categorías de sistema (FR-004) | Ingreso: Cobros de clientes, Aportes de socios · Egreso: Pagos a proveedores, Pagos a empleados, Retiros de socios |
| Validez de presupuestos | 15 días |

**Genérico** (`generic`):

| Elemento | Contenido |
|---|---|
| Etapas (en orden) | Presupuesto (abierta, azul) → Aprobado (abierta, verde azulado) → En curso (abierta, amarillo) → Finalizado (ganado, verde) · Perdido (perdido, gris) |
| Categorías de egreso | Materiales, Mano de obra, Flete, Alquiler, Servicios, Impuestos, Otros gastos |
| Categorías de ingreso | Otros ingresos |
| Catálogo de ejemplo | (ninguno) |
| Condiciones comerciales | (ninguna) |

**Carpintería de aluminio** (`aluminum_carpentry`):

| Elemento | Contenido |
|---|---|
| Etapas (en orden) | Presupuesto (abierta, azul) → Aprobado (abierta, verde azulado) → En producción (abierta, naranja) → Instalación (abierta, violeta) → Finalizado (ganado, verde) · Perdido (perdido, gris) |
| Categorías de egreso | Perfiles de aluminio, Vidrios, Accesorios, Flete, Mano de obra, Alquiler, Servicios, Impuestos |
| Categorías de ingreso | Otros ingresos |
| Catálogo de ejemplo (sin precio) | Ventana corrediza (m²), Puerta de abrir (unidad), Mampara de baño (unidad), Colocación (unidad) |
| Condiciones comerciales | "Seña del 50 % al aprobar el presupuesto y saldo contra entrega." |

Las empresas registradas antes de 002 reciben al migrar la configuración mínima: los nombres por
defecto, las categorías de sistema y las etapas de "Genérico" (sin catálogo ni condiciones).

### Cambios de redacción propuestos

Cada fila reemplaza el texto actual al aprobarse. Lo que no figura, no cambia.

| Dónde | Texto actual | Texto propuesto |
|---|---|---|
| Historia 1, prueba independiente | "…y verlas reflejadas en el tablero de proyectos." | "…y ver el pipeline resultante en la configuración. El tablero de proyectos lo muestra desde la spec 004." |
| Historia 1, escenario 1 | "…**entonces** los proyectos muestran el nuevo pipeline." | "…**entonces** el pipeline de la empresa queda con esos nombres y ese orden (los proyectos lo muestran desde 004)." |
| Historia 1, escenario 2 | (sin cambios) | Se agrega al final: "Cambiar el tipo de una etapa afecta a los proyectos que estén en ella sin modificarlos (P-2)." |
| Historia 1, escenario 3 | "**Dado** una etapa con proyectos, **cuando** se intenta eliminarla, **entonces** el sistema pide moverlos a otra etapa antes de eliminarla." | "**Dado** una etapa, **cuando** el Administrador la elimina, **entonces** sale del pipeline y no se puede recuperar. Si tiene proyectos, el sistema pide moverlos a otra etapa antes de eliminarla (se verifica en 004)." |
| Historia 1, escenario 4 | "…siempre existe al menos una etapa inicial y una de cierre *ganado*." | "…la etapa inicial es la primera y es *abierta*, y existe al menos una de cierre *ganado*. Un cambio que lo rompa (eliminar, mover, cambiar el tipo o crear) se rechaza con un mensaje claro." |
| Historia 1, escenario nuevo 5 | — | "**Dado** una etapa activa con un nombre, **cuando** se intenta crear o renombrar otra con el mismo nombre, **entonces** el sistema lo rechaza." |
| Historia 2, prioridad y prueba | (sin prueba independiente) | Se agrega: "**Prueba independiente**: cambiar 'Proyecto' por 'Obra' y ver 'Obra' en la interfaz." |
| Historia 2, escenario 1 | "…cambia 'Proyecto' por 'Obra' (singular y plural), **entonces** toda la interfaz y los PDF usan 'Obra'." | "…cambia 'Proyecto' por 'Obra' (singular, plural y género), **entonces** toda la interfaz usa 'Obra' con la concordancia correcta ('Nueva obra'). Los PDF lo usan desde 005." |
| Historia 3, escenario 1 | "…**entonces** queda disponible al registrar movimientos." | "…**entonces** aparece entre las categorías activas de ese tipo (los movimientos la usan desde 006). El tipo no se puede cambiar después." |
| Historia 3, escenario 2 | "**Dado** una categoría con movimientos, **cuando** se intenta eliminarla, **entonces** solo se puede **archivar**…" | "**Dado** una categoría, **cuando** el Administrador la **archiva**, **entonces** deja de ofrecerse pero se conserva en el historial, y se puede restaurar. Las categorías no se eliminan." |
| Historia 3, escenario nuevo 4 | — | "**Dado** una categoría activa, **cuando** se intenta crear, renombrar o restaurar otra del mismo tipo con el mismo nombre, **entonces** el sistema lo rechaza." |
| Historia 4, escenario 1 | "…precio de referencia y moneda, **entonces** puede usarlo al armar presupuestos." | "…precio de referencia opcional (importe y moneda), **entonces** aparece en el catálogo activo (los presupuestos lo usan desde 005). Con la unidad 'otra', se indica su nombre (p. ej. 'jornada')." |
| Historia 4, escenario 2 | (sin cambios) | Se agrega: "(Se verifica en 005: el presupuesto copia los datos del ítem.)" |
| Historia 4, escenario 3 | "…cuando se archiva, entonces deja de ofrecerse en presupuestos nuevos." | "…cuando se archiva, entonces deja de ofrecerse en presupuestos nuevos; se puede restaurar." |
| Historia 5 | "Ver 006. La plantilla crea por defecto 'Caja efectivo ARS'." | "*Retirada (P-1)*: se mueve a la spec 006, que crea 'Caja efectivo' en la moneda base de la empresa." |
| Historia nueva 6 | — | "### Historia 6: Condiciones comerciales por defecto (Prioridad: P2) — **Prueba independiente**: definir 30 días y un texto, y verlos como valores por defecto. 1. **Dado** un Administrador, **cuando** define la validez (1 a 365 días) y el texto de las condiciones comerciales, **entonces** quedan como valores por defecto de los presupuestos nuevos (005). 2. **Dado** un cambio de esos valores, **entonces** los presupuestos ya creados no se modifican." |
| Casos borde | — | Se agrega: "Cualquier usuario de la empresa puede leer la configuración; solo el Administrador la modifica (P-5)." y "La plantilla se aplica solo al registrarse; no se reaplica ni se cambia de rubro." |
| FR-001 | "…cada una de tipo *abierta*, *ganado* o *perdido*, con un color." | "…cada una de tipo *abierta*, *ganado* o *perdido*, con un color de una paleta cerrada. La primera etapa es la inicial y es *abierta*; siempre hay al menos una *ganado*." |
| FR-002 | "…nombres singular y plural de: Proyecto, Cliente y Presupuesto." | "…nombres singular, plural y género de: Proyecto, Cliente y Presupuesto." |
| FR-003 | "…crear, renombrar y archivar categorías de ingreso y de egreso." | "…crear, renombrar, archivar y restaurar categorías de ingreso y de egreso. Las categorías no se eliminan y su tipo no cambia." |
| FR-004 | "…Retiros de socios." | "…Retiros de socios. Se pueden renombrar, no archivar. Las specs siguientes pueden agregar categorías de sistema (p. ej. 'Ajuste de arqueo', 006)." |
| FR-005 | "…catálogo de ítems con unidad, precio de referencia y moneda." | "…catálogo de ítems con unidad y precio de referencia opcional (importe en centavos y moneda), que se pueden archivar y restaurar." |
| FR-006 | "…precargan etapas, categorías, nombres y catálogo de ejemplo…" | "…precargan, al registrarse la empresa, etapas, categorías, nombres, catálogo de ejemplo y condiciones comerciales (contenido en Clarificaciones, P-4)…" |
| FR nuevo FR-008 | — | "**FR-008**: El sistema DEBE permitir a cualquier usuario de la empresa leer la configuración y solo al Administrador modificarla." |
| Entidades clave | Etapa, Categoría, Ítem, Etiquetas | "**Etapa (Stage)**: nombre, orden, tipo, color (de paleta); eliminada. **Categoría (Category)**: nombre, tipo, clave de sistema (si es de sistema), archivada. **Ítem de catálogo (CatalogItem)**: nombre, descripción, unidad (y su nombre si es 'otra'), precio de referencia opcional con moneda, archivado. **Nombres personalizados (Labels)**: singular, plural y género por entidad. **Condiciones por defecto (QuoteDefaults)**: validez en días y texto." |
| SC-001 | "…sin cambios de código." | "…sin cambios de código. En 002 se verifica con un escenario automatizado que configura una herrería solo por la API; de punta a punta, cuando existan 004 a 007." |
| Supuestos | — | Se agrega: "No hay configuración de monedas: ARS y USD están siempre disponibles y la moneda base se fija al registrarse (001)." |
