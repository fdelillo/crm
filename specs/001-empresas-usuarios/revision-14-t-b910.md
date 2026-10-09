# Decimocuarta revisión: resultado de T-B910, techo de ADR-005 y revisión del PR #17

**Fecha**: 2026-10-08 · **Autor**: `backend-architect`

**Estado**:

- **Parte A** (T-B910 y techo de ADR-005): *Accepted*, aprobada por el usuario el 2026-10-08 con los
  seis defaults de §8.
- **Parte B** (revisión de código del PR #17): *Accepted*, aprobada por el usuario el 2026-10-08 con los seis defaults (§B-7).

**Origen**:

- T-B910 terminó en `feat/001-backend-phase-9` (PR fdelillo/crm#17, HEAD `371b6b1`). Los resultados
  están en `tasks.md`, "Estado de la implementación", entrada "Fase 9", párrafo "Paso 4 completo —
  T-B910" (tablas M-1 a M-6 de las dos corridas).
- Se aplica la primera regla de decisión de la decimotercera revisión (`revision-13-t-b905.md` §5):
  T-1, T-2, T-3 y `set_role_retry_total = 0` en las dos corridas, y D-a a D-d sin disparar.
- La revisión de código final del PR #17 (HEAD `371b6b1`) dejó tres puntos de diseño. Van en la
  parte B.

**Reabre**:

- Parte A: DD-33 (suma R-f y los números); `plan.md` §12.3, §12.4 (crecimiento), §13, §14.1 S-3,
  §14.2 P-1, §15 R-2 y R-15, §16 y §18; research R-04c y supuesto 15; nota (d) en ADR-005.
- Parte B: reglas (4) y (6, nueva) de T-B801; mutaciones de T-B903 y del paso 0 de la Fase 9;
  autocontrol del tracer de T-B910; entrada de estado de la Fase 9; `plan.md` §16 y §18.

**Reemplaza** la segunda oración de la primera regla de decisión de la decimotercera tanda ("T-B910
se repite con 20.000 roles antes de que `tenants_total` llegue a 5.000"). El texto de esa tanda no
se edita.

**Verificado contra**:

- Parte A: las tablas M-1 a M-6 de las dos corridas en `tasks.md`; `plan.md` (DD-33, §12.1, §12.3,
  §12.4, §13, §14, §15, §16); ADR-005; research R-04c y supuestos. Todos los cálculos salen de esas
  tablas: no se usó ningún dato que no esté ahí.
- Parte B:
  - `cmd/crm/serve_router_test.go`: `isEnvListen` (líneas 301–308), el chequeo de literales de
    `env` (385–433) y el de imports (246–252);
  - `cmd/crm/main.go`: `env` y `run`;
  - `tasks.md`: reglas (1) a (5) y mutaciones de T-B801, T-B903, T-B910, T-B911 y las entradas de
    estado de la Fase 9.

La parte A no cambia código ni tests. La parte B cambia solo código de test (el guard de T-B801 y el
tracer de T-B910). Ninguna de las dos cambia código de producción.

---

# Parte A — T-B910 y techo de ADR-005 (*Accepted*)

## Resumen

- **Qué registra.** Los números de T-B910 en DD-33, §13, R-2, R-15, research (R-04c y supuesto 15)
  y una nota (d) en ADR-005. H-1 queda **confirmada en el comportamiento**, con una ampliación: los
  dos `GRANT` también son lineales, no solo los `SET ROLE`.
- **La lectura del orquestador es correcta en lo esencial**, con tres precisiones (§1): la
  pendiente es 7,4–9,2 ms por cada 1.000 roles (no 9–10); el D-d de la corrida 1 es ruido, no
  evidencia; y Docker Desktop no es necesariamente pesimista **para el techo**, porque la pendiente
  es CPU del backend y una instancia de producción chica puede ser más lenta.
- **Techo (§3).** Con P = 8, T-1 deja de cumplirse alrededor de **10.500 roles** (conservador; rango
  10.500–15.000). Con dos instancias de 8 (P = 16), alrededor de **3.600–6.600**. Los `503` en ráfagas
  recién aparecen hacia 25.000–32.000. El primer `SET ROLE` después de un registro (D-c) y el signup
  quedan lejos: 70.000–90.000 y 65.000–80.000.
- **Qué decide (§4).** El disparador deja de ser "repetir T-B910 con 20.000" (resultado previsible:
  ~180–240 ms contra 140) y pasa a ser una nueva restricción **DD-33 R-f**. Los disparadores se
  miden sobre `tenant_roles_total` y son fracciones del techo conservador:
  - al **50 %** (hoy 5.000), el ADR sucesor;
  - al **75 %** (hoy 8.000), el sucesor en producción.

  Los disparadores se recalculan cuando se mide la pendiente en la clase de instancia de producción
  (al cerrar P-1) y con cada cambio de P.
- **PR #17 (§5).** La parte A no lo bloquea.

## 1. Verificación de la lectura del orquestador

| Afirmación | Verificación | Veredicto |
|---|---|---|
| El `hold` sube 9–10 ms de p50 por cada 1.000 roles, de 11–12 a 81–96 ms | Los extremos son correctos. La pendiente por mínimos cuadrados sobre los 10 bloques de M-1 es 9,2 ms/1.000 en la corrida 1 y 7,4 en la corrida 2. "9–10" vale solo para la corrida 1 | Precisión: **7,4–9,2** |
| `SET tenant` sube de 1,6 a 23–28 ms | Correcto (p50; 2,4–2,9 ms/1.000) | Correcto |
| Es lineal, no cuadrático, y por eso D-d no salta | Correcto en la conclusión. El cociente de p50 entre los bloques 9.001–10.000 y 4.001–5.000 da 1,92 y 1,95. Con costo lineal más una parte fija de 6–9 ms, el modelo predice ~1,9; con costo cuadrático, ~4. Pero el D-d de la corrida 1 (1,46) no prueba nada: su denominador (p95 del bloque 4.001–5.000, 78,6 ms) es atípico, porque el p50 del mismo bloque es 50,1. La linealidad la confirman el p50 y el ajuste, no el D-d | Correcto. D-d con p95 es ruidoso |
| T-1 se cruzaría alrededor de 11.000–13.000 | §3: entre 10.500 y 15.000; estimación conservadora, 10.500 | El rango es algo más ancho y el borde inferior, más bajo |
| Con 20.000 roles fallaría casi con seguridad (~200 ms o más) | §3: 180–240 ms de p95. Solo cumpliría si la pendiente del hardware de producción fuera ~30 % menor que la menor medida | Correcto |
| En la corrida 1, T-1 tuvo 4,6 % de margen | (140 − 133,6) / 140 = 4,6 % | Correcto |
| Docker Desktop en macOS es pesimista respecto de Linux, pero la pendiente probablemente se mantiene | La pendiente es trabajo de CPU dentro del backend de PostgreSQL (recalcular listas de membresía), no viajes de red. Docker Desktop corre PostgreSQL en una VM Linux: lo pesimista es la parte fija (los viajes de red), no la pendiente. En producción, la pendiente depende de la CPU de la instancia, y una instancia gestionada chica puede ser **más lenta** que 8 CPU de una Mac | **Corrección**: para el techo, el entorno no es necesariamente pesimista. La incertidumbre va en los dos sentidos |

## 2. Dónde está el costo con 10.000 roles (H-1)

Desglose de M-2 y M-3 (p50, corrida 1 / corrida 2, en ms):

| Tramo | Sentencias | ms | ¿Retiene el lock? | ¿Crece con N? |
|---|---|---|---|---|
| Antes del *callback* | `BEGIN` 2,1 / 2,0; `SET LOCAL ROLE crm_signup` 29,4 / 24,4 | 31,6 / 26,4 | No | Sí, el `SET ROLE` |
| `provision_tenant_role` | `CREATE ROLE` 3,5 / 3,2; `GRANT crm_tenant` 29,1 / 25,3; `GRANT … TO crm_app` 31,7 / 27,1 (M-3, sin plpgsql) | 64,0 / 52,9 | Sí, desde el `GRANT crm_tenant` | Sí, los dos `GRANT` |
| `AsTenant` | `SET LOCAL ROLE crm_t_…` | 29,7 / 24,8 | Sí | Sí |
| Lectura del email del admin e inserts de empresa, admin, token, outbox, sesión y auditoría | 7 sentencias de ~1 ms | ~7 | Sí | No |
| `COMMIT` | — | 0,6 | Hasta que termina | No |
| **`hold`** (del inicio de `provision` al fin del `COMMIT`) | — | **102,0 / 85,4** | — | ~90 % del total |

**Qué muestra el desglose:**

- **Cuatro operaciones de cada registro crecen linealmente con N**, cada una ~2,4–3,1 ms por cada
  1.000 roles. Las pendientes de M-1 son: `pre_callback` 2,5–3,0; `SET tenant` 2,4–2,9; `provision`
  (dos `GRANT`) 5,2–6,3. Con 10.000 roles, juntas suman ~100–120 ms de los ~113–135 ms del registro
  sin contención (`end_to_end` de M-2).
- **H-1 queda confirmada en el comportamiento:**
  - (a) **Crece linealmente**: sí (§1).
  - (b) **Cada conexión paga una reconstrucción después de un registro**: sí. En M-4, el primer
    `setRole` da 26,0 / 22,6 ms de p50, contra 0,8 en caliente.
  - (c) **Las 10 muestras de calentamiento de T-B905 lo mostrarían**: **no se observó**. En M-6 dan
    1,0 / 0,7 ms de p95, porque no las precede un registro. El costo lo mide M-4.
- **Ampliación: los dos `GRANT` también son lineales.**
  - Es consistente con S-d de la decimotercera revisión: `crm_provisioner` tiene `ADMIN` sobre los N
    roles que creó. Verificar que tiene autoridad para conceder recalcularía su lista de membresías,
    que el cambio anterior invalidó.
  - No está verificado en el código de PostgreSQL y no cambia ninguna decisión.
- **Consecuencia en el camino de lectura (R-2 fuera del registro).**
  - Un `GET /me` justo después de un registro da 48,3 / 41,6 ms de p95, contra 10,1 / 8,2 en reposo.
  - Lo pagan como mucho P requests por registro, uno por conexión.
  - A la escala S-3 (< 20 req/s y pocos registros por día) no mueve el p95 global. Lo movería con
    más de ~7 registros por minuto sostenidos.

## 3. Techo de ADR-005 con estos parámetros

### 3.1 Modelo

Ajuste lineal por mínimos cuadrados sobre los 10 bloques de M-1. N está en miles de roles, en el
punto medio de cada bloque; la retención, en ms.

| | Corrida 1 | Corrida 2 |
|---|---|---|
| `hold` p50 | 6,4 + 9,2·N | 9,2 + 7,4·N |
| `hold` p95 | 14,0 + 10,4·N | 15,3 + 8,6·N |
| Control: p50 que predice la recta con 10.000 roles vs M-2 | 98,3 vs 102,0 | 83,6 vs 85,4 |
| Control: p95 que predice la recta con 10.000 roles vs M-2 | 118,0 vs 133,6 | 101,6 vs 96,3 |

- El p50 predice M-2 con un error de 2–4 %.
- El p95 de M-2 en la corrida 1 queda 13 % por encima de la recta. Esa corrida tiene más ruido: el
  máximo de M-2 es 207 ms.
- Por eso cada estimación usa dos anclas:
  - la recta de p95;
  - el p95 medido en M-2, proyectado con la pendiente de p95 ("anclada").
- La estimación conservadora es la menor de las cuatro.

### 3.2 Techos

| Qué deja de cumplirse | Umbral | `tenant_roles_total` estimado | Confianza |
|---|---|---|---|
| **T-1**: retención p95 < 140 ms, con P = 8 | 140 ms | **10.500–15.000** (conservador: 10.500, corrida 1 anclada) | Media: extrapola hasta 1,5× lo medido, y las dos corridas difieren 25 % en la pendiente |
| `GET /me` justo después de un registro < 50 ms (no es target: §13 mide en reposo) | 50 ms | 10.400–12.500 | Media |
| Ráfaga ≥ P sin `503`: (P − 1) × retención < `lock_timeout` | 286 ms | 25.000–32.000 | Baja: extrapola 2,5–3×. Además, usa la retención aislada como tiempo en el lock (conservador) |
| Signup p95 < 1 s (T-3) | 1.000 ms | 65.000–80.000 | Baja: extrapola 6–8× |
| D-c: primer `setRole` p95 < 250 ms | 250 ms | 70.000–90.000 | Baja |
| **T-1 con P = 16** (dos instancias de 8): target derivado 1000 / 15 ≈ 67 ms | 67 ms | **3.600–6.600** | Media (interpola) |
| T-1 con P = 4: target derivado 1000 / 3 ≈ 333 ms (palanca, no propuesta) | 333 ms | 29.000–37.000 | Baja |

Con 20.000 roles, la retención p95 proyectada es de 180–240 ms.

**Fuentes de incertidumbre, de mayor a menor peso:**

1. **Hardware.**
   - La pendiente es CPU del backend. En producción puede ser mayor o menor (§1, última fila).
   - Es la incertidumbre que más mueve el techo y la única que se puede reducir: midiendo en la
     clase de instancia de producción.
2. **Dos corridas con pendientes de 7,4 y 9,2.** De ahí sale el rango de la tabla.
3. **p95 de 100 muestras (M-2).** El de la corrida 1 tiene un valor atípico de 207 ms.
4. **Linealidad más allá de 10.000 roles.** No está medida. Para T-1 con P = 8 alcanza con
   extrapolar hasta 1,5×.
5. **Carga concurrente real** (autovacuum, otras transacciones). El bench no la tiene; la cubre el
   factor 2 de T-1.

### 3.3 Qué significa

- **Con P = 8, el diseño tiene ~10× de margen sobre la escala supuesta** (S-3: ~1.000 empresas). El
  "margen de 10×" de la decimotercera revisión resulta ser el techo, no un piso.
- **Cruzar T-1 no produce fallas visibles enseguida.**
  - El target tiene un factor 2 sobre el `lock_timeout`.
  - Los `503` en una ráfaga ≥ P recién aparecen hacia 25.000–32.000.
  - Entre esos dos puntos, lo que se pierde es margen.
- **El techo depende de P.**
  - DD-33 R-e ya decía que P es la suma de los pools. Ahora hay un número: con dos instancias de 8,
    el techo baja a 3.600–6.600.
  - Escalar horizontalmente sin recalcular viola T-1 con pocos miles de empresas.
- **La cola de una ráfaga.** `GET /me` durante una ráfaga de 50 da 43,6 / 42,6 ms de p95 (D-b no se
  dispara), pero el máximo es de 3,6 / 3,1 s.
  - Los requests que llegan cuando el pool está ocupado por registros que esperan el lock esperan
    segundos. Es la amenaza de DoS de §10.3, ahora con número.
  - La cola dura más o menos el tamaño de la ráfaga × la retención, así que crece con N.
  - Se registra y no cambia nada. O-4 (semáforo) se reevalúa en la revisión del 50 %.

## 4. Decisión: el disparador de §12.4

### 4.1 Qué falla del disparador vigente

El texto vigente es "Antes de que `tenants_total` llegue a 5.000, repetir T-B910 con 20.000 roles".
Tiene cinco problemas:

1. **El resultado es previsible**: ~180–240 ms contra 140. El disparador se gasta en confirmar lo
   que ya se sabe, y la acción real ("reabrir ADR-005") igual llega en 5.000.
2. **No fija un plazo** para implementar el reemplazo, que cuesta una fase.
3. **Mide en el entorno del bench**, no en el hardware de producción, que es la incertidumbre más
   grande.
4. **Ignora P**: con dos instancias, el techo queda por debajo de 5.000.
5. **Mira `tenants_total`**, pero el costo depende de los roles `crm_t_*` del clúster
   (`tenant_roles_total`), sobrantes incluidos (restore parcial, S-6).

### 4.2 Opciones

| Opción | Qué | Costo hoy | Costo cuando se dispara | Trade-off |
|---|---|---|---|---|
| D1 (vigente) | Con `tenants_total` en 5.000, T-B910 con 20.000 roles | 0 | ~1 h de máquina, con resultado previsible → ADR sucesor y fase | Los cinco problemas de §4.1 |
| **D2 (elegida)** | Techo explícito (DD-33 R-f) y dos disparadores sobre `tenant_roles_total`, como fracciones del techo conservador. **50 % (hoy 5.000)**: ADR sucesor *Proposed* y fase planificada. **75 % (hoy 8.000)**: sucesor en producción, o el usuario decide seguir con un ADR. Se recalculan al medir en la clase de instancia de producción (P-1) y con cada cambio de P. Una alerta del monitoreo sobre esos dos valores | Solo documentos | Una regla de alerta al desplegar. ~1 h de máquina al cerrar P-1: el bench tiene que aceptar un DSN externo, que es código de test con build tag `bench`. Una revisión al 50 % y la fase al 75 % | Depende de que alguien configure la alerta (sin código: `tenant_roles_total` ya está en `/debug/vars`). Al 50 % queda ~2× de margen para hacer una fase |
| D3 | D2 con umbrales más bajos (3.000 / 5.000) | Igual que D2 | Igual que D2, pero antes | Más margen para un crecimiento rápido, y quizá trabajo innecesario si producción resulta más rápida. S-3 no lo justifica |
| D4 | Escribir ya el ADR sucesor (*Proposed*), sin implementarlo | Una revisión | — | Decide con datos de un hardware que no es el de producción y queda viejo. La escala supuesta está a 10× |
| D5 | Implementar B ya | Una fase (toca las Fases 0–3, 8 y 9) | — | Revierte la decisión del usuario por un problema que está a 10× de la escala supuesta. Contradice la regla aprobada (O-1) |

**Por qué D2:**

- **Los umbrales salen del techo, no son números sueltos.** Si la medición en producción cambia la
  pendiente, cambian con ella.
- **50 % deja margen para el reemplazo**: ~5.000 empresas para una revisión, una decisión y una
  fase. **75 % deja ~25 %** para la incertidumbre de la estimación.
- **`tenant_roles_total` es la variable que gobierna el costo.** En operación normal coincide con
  `tenants_total`. Si hay roles sobrantes, la sobreestima y el disparador salta antes: queda del
  lado seguro.
- **La medición en la clase de instancia de producción reduce la incertidumbre más grande** (§3.2,
  punto 1).
  - Se hace una sola vez, cuando se cierra P-1.
  - Se hace contra una instancia **descartable**, porque deja 10.000 roles en el clúster. Nunca
    contra el clúster de producción (S-6).
  - Si el proveedor no da superusuario, M-3 queda "no medido". M-1 y M-2 alcanzan para la
    pendiente.

**Mejoras que estiran el techo sin cambiar de modelo.** No se proponen ahora; se evalúan en la
revisión del 50 %.

- **O-5. Reordenar `provision_tenant_role`**: conceder `<rol> TO crm_app` antes del `GRANT
  crm_tenant`.
  - Según la nota de ADR-005, `GRANT <rol> TO x` bloquea el rol concedido. El `GRANT` a `crm_app`
    bloquearía el rol nuevo, que nadie más espera.
  - Sacaría del lock ~27–32 ms (~30 % de la retención) y subiría el techo de T-1 en ~40–50 %.
  - No está verificado. Toca la función de la Fase 1 (migración nueva) y no cambia R-2 en el camino
    de lectura.
- **O-6. Bajar P.**
  - Con P = 4, el target derivado sube a 333 ms y el techo a ~29.000–37.000.
  - Es solo configuración, pero cambia R-e y el target de §13. Además, con un P más chico alcanza
    una ráfaga más chica para ocupar todo el pool.
- **O-4 (semáforo)** no aplica: D-b no se disparó.

**Una consideración para P-1.** El hosting todavía no está elegido.

- Si el horizonte del producto supera las ~5.000 empresas, aplicar B **antes** de salir a producción
  cuesta el mismo código y evita retirar roles vivos.
- Con la escala supuesta, no hace falta. Queda escrito en P-1 para quien lo cierre.

## 5. ¿La parte A bloquea el merge del PR #17?

**No.**

- T-B910 cumplió la regla fijada antes de medir, y la condición de salida a producción de ADR-005
  está cumplida.
- La parte A no cambia código, tests, contrato ni `data-model.md`. Cambia documentos y agrega una
  regla de operación: la alerta, que se configura al desplegar (P-1).
- El techo está a ~10× de la escala supuesta, y la acción más cercana (el 50 %) está a miles de
  empresas.
- Lo que sí condiciona el despliegue ya estaba decidido: `pool_max_conns=8` explícito en
  `DATABASE_URL` (R-e) y una sola instancia (S-1). Con R-f, agregar una segunda instancia exige
  recalcular el techo antes.

Los documentos van en la rama del PR #17, antes del merge (pregunta 6 de §8).

## 6. Supuestos

| Id | Supuesto | Si es falso |
|---|---|---|
| S-e | La pendiente es trabajo de CPU del backend y escala con la CPU de la instancia | Si la instancia de producción es mucho más lenta, el techo baja. Lo detecta la medición de P-1 (research, supuesto 16) |
| S-f | El costo sigue siendo lineal entre 10.000 y 15.000 roles | Solo afecta el borde superior del rango de T-1 |
| S-g | `tenant_roles_total` ≈ cantidad de roles de los que `crm_app` es miembro | Con roles sobrantes, la sobreestima: dispara antes (lado seguro) |
| S-h | Docker Desktop no distorsiona la pendiente más allá de la diferencia de CPU | Mismo efecto que S-e; se resuelve con la misma medición |

## 7. Invariantes

- **Se preservan todas**: INV-14, INV-26 e INV-27 no cambian.
- **DD-33** suma R-f, una restricción de operación sin código.
- **ADR-005** no cambia de decisión: se le agrega la nota (d).
- **La regla de decisión de la decimotercera tanda** se aplica tal como se fijó. Solo se reemplaza
  el disparador de la re-medición, que esa tanda había dejado para más adelante.

## 8. Lo que aprobó el usuario (2026-10-08, los seis defaults)

| # | Pregunta | Default aprobado |
|---|---|---|
| 1 | ¿Se registran los números de T-B910 en DD-33, §13, R-2, R-15, research y ADR-005 (nota (d)), con H-1 confirmada en el comportamiento? | Sí |
| 2 | ¿DD-33 suma R-f: con P = 8, techo de ~10.500 roles (conservador; rango 10.500–15.000), que se recalcula con la pendiente de producción y con cada cambio de P? | Sí |
| 3 | ¿Disparadores D2 sobre `tenant_roles_total`, con alerta: al 50 % del techo (hoy 5.000), ADR sucesor; al 75 % (hoy 8.000), sucesor en producción o un ADR del usuario para seguir? | D2 |
| 4 | ¿Al cerrar P-1, se mide T-B910 (M-1 y M-2) en una instancia descartable de la clase de producción, sin que sea condición de salida? | Sí, sin que sea condición |
| 5 | ¿Una segunda instancia (o un pool mayor que 8) exige recalcular el techo antes de desplegarse? | Sí |
| 6 | ¿Los cambios de documentos van en el PR #17 antes del merge? | Sí, en el PR #17 |

## 9. Cambios en los documentos (parte A)

Los pares exactos "texto actual → texto nuevo" se entregaron al orquestador, que los aplicó como
*Accepted*. Archivos tocados:

- **`plan.md`**: encabezado y tabla de artefactos; DD-33 (R-f, los números y el trade-off); §12.3;
  §12.4; §13; §14.1 S-3 y §14.2 P-1; §15 R-2 y R-15; §16; §18.
- **`tasks.md`**: encabezado y título de "Estado de la implementación"; entrada de la decimocuarta
  revisión; resultado y repetición de T-B910; condición de salida a producción.
- **`research.md`**: encabezado, nota (b) en R-04c, supuesto 15 y supuesto 16 nuevo.
- **`docs/adr/005-…`**: nota (d).

## 10. Hallazgos de documentación (parte A)

1. **S-3** ponía el umbral para revisar DD-33 en "decenas de miles de empresas". Con estos números,
   el techo de T-1 está en ~10.500.
2. **§12.4 y §16** usaban `tenants_total` como variable del crecimiento, pero el costo depende de
   `tenant_roles_total`.
3. **`plan.md` §18**, "Estado de la implementación", decía que la Fase 9 estaba "detenida en
   T-B905".
4. **`tasks.md`**: el título "Estado de la implementación (2026-10-05)" tenía la fecha desactualizada.
5. **ADR-005, consecuencia 1**, nombra a T-B905 como la medición previa a salir, pero la que decidió
   fue T-B910. Como el ADR está aceptado, no se edita: lo aclara la nota (d).
6. **D-d se calcula con el p95 de bloques de 1.000 y es ruidoso**: en la corrida 1 dio 1,46 por un
   denominador atípico.
   - En una repetición conviene reportar también el cociente de p50 y la pendiente de M-1.
   - No hace falta código: los dos salen del CSV del tracer.

## 11. Matriz de mantenimiento (agregado de la parte A)

| Cuando cambies… | También tenés que actualizar… |
|---|---|
| `pool_max_conns` o la cantidad de instancias | **Antes de desplegar**: DD-33 R-e y R-f (techo y disparadores) + §12.4 + la alerta |
| La clase de instancia o la versión de PostgreSQL de producción | Medir la pendiente (T-B910 M-1 y M-2 contra una instancia descartable) + R-f + §12.4 + la alerta |
| `tenant_roles_total` cruza un disparador | §12.4: revisión del arquitecto (50 %) o fase del sucesor (75 %) |
| La transacción de registro (sentencias, orden, sembrado de 002) | Re-medir T-B910 (cambia la pendiente o la parte fija) + R-f |

---

# Parte B — Revisión del PR #17 (*Accepted*)

Tres puntos de diseño de la revisión de código final del PR #17 (HEAD `371b6b1`), cada uno
verificado contra el código de la rama. Los otros dos hallazgos de esa revisión son de
implementación; B-5 dice por qué no cambian ninguna tarea, salvo una frase del autocontrol de
T-B910.

## B-1. Regla (4) de T-B801: el campo `listen` por su objeto, y ningún otro tipo con la forma de `env`

### Qué pasa

El guard reconoce el campo por el **nombre del tipo receptor**:

- `isEnvListen` (`serve_router_test.go`, 301–308) exige que `selection.Recv()` sea el tipo con
  nombre `env` de `cmd/crm` y que el campo se llame `listen`.
- El chequeo de literales (385–433) solo mira los literales cuyo tipo es `env`.
- En el literal de `run`, la rama de `new` (línea 426) solo comprueba que el resultado sea
  `*net.ListenConfig`.

Formas que compilan y que el guard deja pasar:

| Id | Forma | Por qué pasa |
|---|---|---|
| (j) | Campo promovido: `w := struct{ env }{e}`, `w.listen = otro`, `e = w.env` | El receptor de la selección es `struct{ env }`, no `env` |
| (k) | Tipo definido: `type env2 env`, `e2 := env2(e)`, `e2.listen = otro`, `e = env(e2)` | El receptor es `env2` |
| (l) | Conversión desde un literal anónimo: `e = env(struct{ …; listen … }{…, listen: otro})` | El literal no es de tipo `env`; su clave `listen` es un campo de ese struct anónimo |
| (m) | `new(net.ListenConfig{Control: c}).Listen` en el literal de `run` | Go 1.27 (la versión del proyecto) acepta una expresión en `new`, y el resultado sigue siendo `*net.ListenConfig` |
| (n) | Asignación **sin** conversión: `var s struct{ …los campos de env… }`, `s.listen = otro`, `e = s` | Es asignable porque los subyacentes son idénticos y uno de los dos tipos no tiene nombre (asignabilidad de la especificación de Go). No hay literal ni conversión: **la propuesta del revisor no la cierra** |
| (o) | Conversión de puntero: `p := (*struct{ … })(&e)`, `p.listen = otro` | El receptor es el struct anónimo, que tiene sus propios objetos de campo |

### Decisión

Se adopta la propuesta del revisor y se generaliza su segunda mitad para cerrar también (n) y (o).

1. **El campo se reconoce por su objeto.**
   - El guard toma el `*types.Var` del campo `listen` del `Underlying()` de `env`.
   - Lo compara con `info.Selections[sel].Obj()` en cada selección y con `info.Uses[clave]` en cada
     clave de un literal compuesto.
   - Así cuentan también los accesos por un campo promovido y por un tipo definido a partir de
     `env`, porque comparten ese objeto. Cierra (j) y (k); lo verifican esas dos mutaciones.
2. **No hay conversiones a `env` ni a `*env`, ni desde ellos**: ningún `ast.CallExpr` cuyo `Fun`
   sea un tipo (`info.Types[Fun].IsType()`) las hace. Es la propuesta del revisor: da un mensaje
   claro y cubre `env(x)` venga de donde venga.
3. **Ningún otro tipo tiene la forma de `env`.** La declaración de `env` es la única declaración
   de tipo, y su `struct` el único tipo `struct` escrito en el código (incluidos los anónimos, los
   que están detrás de un puntero y los de una restricción de tipo), cuyo subyacente es idéntico
   (`types.Identical`) al de `env`.
   - El revisor proponía prohibir los **literales** de ese tipo. Prohibir el **tipo** cierra
     además (n) y (o), que no usan literal, y vuelve a cerrar (k) y (l) por otro camino.
4. **El argumento de `new` tiene que ser un tipo** (`info.Types[arg].IsType()`), no una expresión.
   Cierra (m).

### Alternativas descartadas

- **Enumerar los receptores** que se aceptan (`struct{ env }`, tipos definidos…). Una lista se
  esquiva con la próxima forma que nadie listó; es la misma razón por la que la regla (5) parte de
  `Info.Uses`.
- **Cambiar la costura**: pasar `listen` como parámetro de `runServe` en lugar de un campo de
  `env`. Un parámetro también se puede reasignar adentro, así que el guard necesitaría reglas
  equivalentes. Además cambia la costura aprobada en la decimotercera revisión, en la última fase.
- **Aceptar el riesgo residual sin reglas nuevas.** Las formas (j), (k) y (n) son plausibles en un
  refactor de buena fe (envolver `env`, definir una variante para tests). Cerrarlas cuesta unas
  líneas del guard y una mutación cada una.

## B-2. `unsafe`, `reflect` y cgo en `cmd/crm`: regla (6) nueva

### Qué pasa

El análisis de las reglas (1) a (5) supone que todo acceso a un valor pasa por el sistema de tipos.

- **`unsafe`** es el hueco real: `*(*func(…) …)(unsafe.Add(unsafe.Pointer(&e), off)) = otro`
  reescribe el campo sin ninguna selección que el guard vea. `unsafe` además habilita
  `//go:linkname`.
- **`reflect`** solo no alcanza para escribir un campo no exportado: el `reflect.Value` de un campo
  obtenido por un nombre no exportado no es asignable (`CanSet` da `false` y `Set` entra en
  pánico). Combinado con `unsafe` (`reflect.NewAt`), sí alcanza.
- **cgo** (`import "C"`) escribe memoria arbitraria.

Hoy ningún archivo de `cmd/crm` importa ninguno de los tres (verificado con Grep).

### Decisión: regla (6) nueva

Ningún archivo no-test de `cmd/crm` importa `unsafe`, `reflect` ni `C`.

- **Por qué una regla nueva y no la (2).** La (2) existe porque los handlers y los servidores los
  arma `internal/app`. La (6) existe porque el guard solo es válido sin escapes de memoria ni de
  reflexión. Con números distintos, cada violación explica su motivo. El costo es el mismo: una
  condición más en el mismo recorrido de imports.
- **Por qué `reflect`, si solo no alcanza para escribir.** Es defensa en profundidad sin costo
  (`cmd/crm` no lo necesita), y así el guard no depende de un razonamiento sobre `CanSet` que
  nadie va a volver a verificar.
- **Alternativa descartada: una regla `depguard` del lint para `cmd/crm`.** Pondría la regla en un
  segundo lugar, lejos de las demás reglas de composición y sin sus mutaciones.
- **Costo**: ~5 líneas del guard y dos mutaciones. Ningún cambio de producción.

### Modelo de amenaza del guard (se deja escrito)

- **Qué protege.** El guard existe para que el invariante (un solo router, servido solo en
  `HTTP_ADDR`; listeners reales en las direcciones configuradas) no se pueda romper por accidente
  ni con un refactor de apariencia razonable.
- **Qué no protege.** No es una defensa contra código escrito para esquivarlo: archivos de
  ensamblador, un `//go:linkname` con `unsafe` permitido en otro paquete, o editar el propio guard.
  Eso lo cubre la revisión de código.
- **Regla para lo que siga.** Un escape nuevo recibe una regla si puede aparecer en código
  plausible; si no, va a la revisión de código. Con B-1 y B-2, las formas plausibles conocidas
  quedan cerradas.

## B-3. Edición de Codex en T-B903 (`8ca9dfb`): se ratifica

**Qué pasó.**

- En `8ca9dfb`, el desarrollador reescribió la lista de mutaciones de T-B903:
  - `Handler: nil` pasó a atribuirse al chequeo explícito de `srv.Handler`, no a la sonda;
  - se agregó la mutación "delegar el mux privado en `http.DefaultServeMux`".
- También corrigió la fila 3 del paso 0 (tabla de la duodécima revisión) de la misma forma.
- La entrada de estado (párrafo "Cambios autorizados…") dice que la nota de `Handler nil` la
  autorizó expresamente el usuario. La mutación agregada va más allá de lo que esa frase nombra.
- No puedo verificar desde los documentos qué autorizó el usuario. La ratificación vuelve
  irrelevante esa pregunta.

**Verificación técnica: el contenido es correcto.**

- Un `Handler` nil en `NewMetricsServer` lo detecta la primera fila de T-B903 ("`Handler` no nil y
  distinto de `http.DefaultServeMux`"), no la sonda.
- La sonda (una ruta registrada en el `http.DefaultServeMux` original) detecta otra cosa: que el
  mux privado delegue en el global. Esa mutación la ejecutó el paso 3 de la reanudación ("sonda
  200, falla").

**Decisión.**

- Se ratifica el contenido, con el texto exacto de los pares TB4 y TB5, que agrega las comillas de
  código y nombra la fila que falla.
- **Regla, repetida para que quede escrita**: el desarrollador no edita texto de diseño
  (descripciones de tareas, reglas, mutaciones). Si encuentra uno incorrecto, lo reporta en su
  entrada de estado y lo corrige el arquitecto.

## B-4. Entrada de estado: "checkpoint aprobado"

La entrada de la Fase 9 dice "completa; checkpoint aprobado tras la decimotercera revisión". Eso
se escribió **antes** de la revisión de código, y aprobar no le corresponde al desarrollador: la
aprobación es el merge del PR por el usuario.

**Decisión** (el texto exacto está en el par TB6):

- **Ahora**: "checkpoint completo en `371b6b1`, pendiente de los ajustes de la revisión del PR
  fdelillo/crm#17 (decimocuarta revisión, parte B)".
- **Al terminar los ajustes**, el desarrollador reemplaza ese paréntesis por "checkpoint completo en
  `<SHA>`, después de la revisión del PR fdelillo/crm#17". Nunca escribe "aprobado".

## B-5. Los otros dos hallazgos (de implementación)

- **`TestPhase9RunServe/cancel-sending` intermitente** (lee `pg_stat_activity` una sola vez).
  - T-B911 ya pide que el test verifique que `runServe` no vuelve "durante al menos 500 ms" mientras
    el SMTP retiene. Una lectura única no cumple ese "durante".
  - El arreglo (observar la condición con reintentos hasta un plazo, sin `sleep` fijo) es de
    implementación. La tarea no cambia.
- **El tracer no exige el conjunto completo de etiquetas por muestra.**
  - No cambia los resultados de T-B910: en las dos corridas, M-2 tiene n = 100 en cada una de las
    14 etiquetas, así que las muestras reportadas estaban completas.
  - Para una repetición (P-1), el autocontrol tiene que exigirlo. Se agrega una frase a la tarea
    (par TB9) para que el código y la tarea digan lo mismo.

## B-6. ¿La parte B bloquea el merge del PR #17?

Va **dentro** del PR #17, antes del merge, como pidió el orquestador.

- Son cambios de test: el guard y el tracer.
- Dejar en `main` escapes conocidos del guard le quita el sentido a T-B801.
- No requiere repetir el bench ni cambiar código de producción.
- Alternativa: un PR posterior solo de tests. Se descarta porque deja `main` con el hueco conocido
  entre los dos PR.

## B-7. Lo que tiene que aprobar el usuario (parte B)

| # | Pregunta | Default |
|---|---|---|
| B1 | ¿Regla (4) ampliada: el campo por su objeto, sin conversiones de `env`, ningún otro tipo con su mismo subyacente y `new` solo con un tipo, con las mutaciones (j) a (o)? La alternativa es solo la propuesta del revisor, que deja abiertas (n) y (o) | Sí, ampliada |
| B2 | ¿Regla (6) nueva: sin `unsafe`, `reflect` ni `C` en los archivos no-test de `cmd/crm`? Las alternativas son sumarlo a la regla (2) o una regla `depguard` | Sí, regla (6) |
| B3 | ¿Se deja escrito el modelo de amenaza del guard (contra accidentes y refactors plausibles, no contra código hecho para esquivarlo)? | Sí |
| B4 | ¿Se ratifican las mutaciones de T-B903 y de la fila 3 del paso 0 tal como las corrigió `8ca9dfb`, con el texto de TB4 y TB5? La alternativa es revertirlas | Ratificar |
| B5 | ¿Texto de estado "checkpoint completo en `371b6b1`, pendiente de…", que el desarrollador actualiza con el SHA final y nunca convierte en "aprobado"? | Sí |
| B6 | ¿La parte B se implementa en el PR #17 antes del merge? La alternativa es un PR posterior | Sí, en el PR #17 |

## B-8. Invariantes y cambios de documentos (parte B)

- **Invariantes.** INV-22 y DD-22 se preservan: el guard queda más estricto sin cambiar lo que
  garantiza. Ninguna firma de §11.1 cambia.
- **`tasks.md`**:
  - encabezado de la decimocuarta revisión;
  - entrada de estado de la decimocuarta revisión, con el orden de trabajo de la parte B;
  - regla (4) de T-B801, la regla (6) nueva y sus mutaciones;
  - mutaciones de T-B903 y fila 3 del paso 0 (ratificadas);
  - paréntesis de la entrada de la Fase 9;
  - autocontrol de T-B910.
- **`plan.md`**: encabezado de la decimocuarta revisión, §16 (reglas (1) a (6)) y §18 (parte B).
- **No cambian**: `research.md`, los ADR, el contrato ni `data-model.md`.

## B-9. Matriz de mantenimiento (agregado de la parte B)

| Cuando cambies… | También tenés que actualizar… |
|---|---|
| Los campos de `env` o cómo se construye | Reglas (4) y (6) de T-B801 y sus mutaciones, en el mismo PR (el guard toma el subyacente de `env` del chequeo de tipos, así que un campo nuevo no rompe la regla, pero sí puede requerir una mutación nueva) |
| Los imports de `cmd/crm` | Reglas (2) y (6) de T-B801 |
| Un texto de diseño que el desarrollador encuentra incorrecto | Se reporta en la entrada de estado; lo corrige el arquitecto en una revisión |
