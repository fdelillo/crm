# Decimotercera revisión: T-B905, la medición del registro y pendientes del PR #17 — *Accepted*

**Fecha**: 2026-10-08 · **Autor**: `backend-architect` · **Estado**: *Accepted*, aprobada por el usuario el 2026-10-08

**Origen**:

- La Fase 9 se detuvo en T-B905 (`feat/001-backend-phase-9`, PR fdelillo/crm#17, borrador, HEAD
  `da0a416`). El reporte del desarrollador está en `tasks.md`, "Estado de la implementación",
  entrada "Fase 9", párrafo "T-B905 ejecutado, falló".
- La revisión de código del PR #17 (`code-reviewer`), en paralelo, dejó tres pendientes de
  diseño. Van en la parte B.

**Reabre**: ADR-005 (nota del 2026-09-30 y "Recomendación del arquitecto", punto 2), DD-33 R-b,
`plan.md` §13 y T-B905 (parte A); `plan.md` §11.1 y la regla (4) de T-B801 (parte B).

**Verificado contra**:

- Parte A: `internal/testsupport/phase9bench/bench_test.go`, `internal/tenant/service.go`
  (`Register`), `db/migrations/00002_tenant_functions.sql` (`provision_tenant_role`),
  `internal/platform/db/db.go` (`run`, `setRole`).
- Parte B: `cmd/crm/serve.go` (`runServe`, pool de runtime), `cmd/crm/main.go` (`env`),
  `internal/app/{periodic,readiness,server}.go`, `internal/identity/cleanup.go`,
  `internal/platform/outbox/stats.go`, `internal/tenant/{inventory,reprovision}.go`.
- pgx v5.11.0 del *module cache*: `tx.go`, `conn.go`, `pgxpool/tracer.go`, `pgxpool/pool.go`.
- research R-04c y R-28.

En esta revisión no cambia ninguna línea de código. Los cambios de código que decide la parte B
los hace el desarrollador en la rama de la Fase 9.

---

## Resumen

**Diagnóstico (parte A).** La corrida 1 no demuestra que ADR-005 falle ni que cumpla.

- **Qué midió.** El target medía la transacción de registro **incluida la espera por el lock** de
  `crm_tenant`. Con un pool de 8 conexiones, lo medido es ≈ 8 × la retención del lock, sea la
  ráfaga de 10 o de 50 (por eso dan casi igual). El `code-reviewer` llegó a lo mismo por su lado:
  los números son correctos según la definición vigente; el problema es la definición.
- **Lo que se infiere, sin medirlo.** La retención sale de 84–91 ms.
- **La señal que no se explica.** El registro HTTP **sin contención** tarda 236 ms de p95, más de
  lo que suman argon2 y esa retención. La hipótesis que lo explicaría (H-1) es el costo R-2:
  recalcular las membresías de `crm_app` con miles de roles después de cada registro.
- **De quién es el error.** Es propio: la tercera revisión definió el target mezclando la causa
  (la retención, que controla el diseño) con el efecto (la espera, que depende del tamaño del pool
  y de la ráfaga).
- **Hallazgo adicional.** El tamaño del pool de producción no está fijado (default de pgxpool: el
  mayor entre 4 y la cantidad de CPU), y la cuenta de los `503` depende de él.

**Decisión recomendada (parte A).** No elegir todavía entre mantener el diseño, el *pool* de
roles pre-creados o la alternativa B: los datos no alcanzan. En cambio:

- **(a) Redefinir el target en dos.** La retención sin contención, con 10.000 roles, debe tener
  p95 < 140 ms (más estricto que los 250 ms anteriores leídos como retención). Una ráfaga de 10
  registros debe terminar con 0 `503`.
- **(b) Fijar el pool de producción** (DD-33 R-e).
- **(c) Medir con T-B910.** Desglose por sentencia, curva con N, primer uso después de un
  registro, ráfagas repetidas. Dos corridas, sin cambios de producción.
- **(d) Fijar antes de medir qué se decide con cada resultado.** Si la retención cumple, se
  mantiene ADR-005 con números y con un disparador de re-medición a las 5.000 empresas. Si no
  cumple, se aplica la cláusula de ADR-005: un ADR que lo reemplace, con B recomendada.
- **(e) No bloquear la Fase 9.** Se cierra con T-B910 reportada, cualquiera sea el resultado. Lo
  que queda condicionado a T-B910 es la salida a producción.

**Parte B (pendientes del PR #17).**

- **(B-1)** La API exportada que falta en §11.1 se documenta tal como está, con dos notas de
  diseño aceptadas.
- **(B-2)** Regla (5) nueva en el guard de T-B801: las cinco funciones que el guard protege solo
  se pueden usar como llamada directa. Así deja de poder esquivarse con valores de función.
- **(B-3)** Costura `env.listen` en `cmd/crm`, regla (4) ajustada para exigirla, y T-B911 nueva
  con los tests de `runServe` que matan las dos mutaciones que hoy sobreviven.

**Qué tiene que aprobar el usuario** (§9): el diagnóstico y los dos targets nuevos; DD-33 R-e;
T-B910 con sus reglas; el tratamiento de la Fase 9; las reglas (4) y (5) del guard con la costura
`env.listen`; y T-B911.

---

# Parte A — T-B905

## 1. Qué midió la corrida 1

Pasos de `Register` (`internal/tenant/service.go`) y de `db.run` (`internal/platform/db/db.go`),
en orden. La medición del bench (`transactionMeter`) arranca al entrar al *callback* y termina
cuando `InSystemTx` vuelve, después del `COMMIT`.

| Paso | Dónde | ¿Dentro de la medición? |
|---|---|---|
| Espera por una conexión del pool | `pool.Begin` en `db.run` | **No** |
| `BEGIN` y `SET LOCAL ROLE crm_signup` (con la lectura de catálogo de DD-34) | `db.run` → `AsSystem` | **No** |
| `SetSignupLockTimeout` | *callback* | Sí |
| `provision_tenant_role`: `CREATE ROLE`, **`GRANT crm_tenant`** (toma el lock; acá se espera) y `GRANT <rol> TO crm_app` | *callback* | Sí, **incluida la espera por el lock** |
| `AsTenant` (`SET LOCAL ROLE crm_t_…`, con la lectura de catálogo) | *callback* | Sí |
| Inserts: empresa, admin, sembrado (vacío en 001), token y outbox, sesión, auditoría | *callback* | Sí |
| `COMMIT` (suelta el lock) | `db.run` | Sí |

Las tandas corrieron una sola vez, sin calentamiento, justo después de cargar las 10.000 empresas
(~13 minutos de carga secuencial: del orden de 78 ms por registro en promedio, con `BEGIN` y
cambio de rol incluidos; el promedio no dice cómo crece con N).

### Modelo de cola, y la lectura del orquestador

La lectura del orquestador es correcta en la conclusión. El mecanismo necesita una precisión.

- **Correcto**:
  - 10 y 50 dan casi igual porque el pool limita cuántas transacciones compiten.
  - Con 10 muestras, el p95 es el máximo: el índice es `(95·10 + 99)/100 − 1 = 9`, el último.
  - Para cumplir < 250 ms con esa cola, cada transacción tendría que durar ≲ 31 ms (250 / 8).
- **Precisión: la cola que entra en la medición es la del lock, no la del pool.**
  - Con P = 8, hasta 8 transacciones tienen el *callback* abierto a la vez: una retiene el lock y
    las otras 7 esperan **dentro** de `provision_tenant_role`.
  - De la 9.ª en adelante esperan en el pool, fuera de la medición. Cuando entran, encuentran 7
    adelante.
  - Por eso lo medido es ≈ P × retención, sin importar el tamaño de la ráfaga. Y la medida es
    optimista para el usuario: no incluye la espera en el pool.
- **Inferencia, no medición**: 672,8 / 8 ≈ 84 ms y 730,2 / 8 ≈ 91 ms de retención.

## 2. Diagnóstico

### 2.1 El target estaba mal definido

T-B905 decía que la duración de la transacción "acota por arriba el tiempo que se retiene el
lock". Es cierto. El error fue ponerle el target a esa cota **medida con concurrencia**, donde
incluye la espera. Ninguna de las dos lecturas posibles era coherente:

| Lectura | Qué exige en realidad | Problema |
|---|---|---|
| La de T-B905: duración de la transacción con 10 simultáneos < 250 ms | Retención ≲ 31 ms con P = 8 (≲ 25 ms si entraran las 10) | Nadie derivó ese número, y además depende de P, que en producción no está fijado (§2.4) |
| La del paréntesis de §13: "tiempo que retiene el lock" < 250 ms | Retención < 250 ms | Con P = 8, una ráfaga que ocupe el pool espera hasta 7 × 250 = 1,75 s: al borde del `lock_timeout` de 2 s. No es compatible con "0 `503`" |

### 2.2 Lo que los datos sí dicen

- 0 `503` con 10 y con 50, coherente con el modelo: la cola dentro del lock nunca supera
  P − 1 = 7, y 7 × ~90 ms ≈ 630 ms < 2 s.
- Lecturas, login y signup en reposo cumplen §13 con margen: 10, 8, 54 y 236 ms contra 50, 50,
  400 y 1000.
- `set_role_retry_total` = 0: la lectura de catálogo de DD-34 alcanzó con 10.000 roles.

### 2.3 Lo que no dicen, y por qué no alcanza para decidir

1. **La retención aislada no se midió.** Los 84–91 ms salen de un modelo.
2. **El registro sin contención es más caro de lo que explica ese modelo.**
   - `POST /auth/signup` (60 muestras secuenciales) dio 236 ms de p95.
   - El login (verificación argon2id más dos transacciones) dio 54 ms, así que argon2 es del orden
     de 40–50 ms.
   - Quedan ~185 ms para el resto del registro, sin espera. Es más que los 84–91 ms de retención
     inferida: algo **fuera del *callback*** cuesta del orden de 100 ms.
3. **Hipótesis H-1** (no verificada en el código de PostgreSQL; la confirma o la descarta el
   desglose de T-B910).
   - **Qué pasaría.** Cada registro cambia `pg_auth_members` tres veces: la membresía con
     `ADMIN` que PostgreSQL 16+ le da a `crm_provisioner` sobre el rol que crea, `GRANT crm_tenant`
     y `GRANT … TO crm_app`. Ese cambio invalida en **todas** las conexiones la lista cacheada de
     roles de los que `crm_app` es miembro. El siguiente `SET ROLE` de cada conexión la
     reconstruye, y esa lista tiene más de 10.000 entradas: costo proporcional a N.
   - **Dónde pegaría en el registro.** Al menos dos veces: en el `SET ROLE crm_signup` (fuera de
     la medición de la corrida 1) y en `AsTenant` (dentro, con el lock tomado).
   - **Antecedentes verificados.** En PostgreSQL 16 se reportó `GRANT` muy lento con miles de
     roles por `roles_is_member_of()`. PostgreSQL 17 agregó un filtro de Bloom a ese cálculo
     (commit d365ae7): eliminó la parte cuadrática, no la lineal. Es R-2 del plan ("cada registro
     invalida la caché de membresías de todos los backends").
   - **Si H-1 es cierta:**
     - (a) la retención crece linealmente con la cantidad de empresas;
     - (b) después de cada registro, **cada** conexión del pool paga una reconstrucción en su
       siguiente request (el `GET /me` de 10 ms se midió en reposo y no lo ve);
     - (c) el `SET LOCAL ROLE` de 1,087 ms se midió en reposo y **descartando las 10 primeras
       muestras como calentamiento** (`if i >= 10`): justo las que mostrarían la reconstrucción.
4. **No se midió la espera en el pool ni su efecto sobre otros requests** durante una ráfaga.
   Mientras esperan el lock, los registros retienen conexiones del pool. §10.3 lo lista como
   amenaza de DoS, pero ningún número lo acota.
5. **Una sola corrida y 10 muestras** para el único target que falló.

### 2.4 Hallazgo de diseño: el tamaño del pool de producción no está fijado

`cmd/crm/serve.go` abre el pool con `pgxpool.New(ctx, cfg.DatabaseURL)`. Sin `pool_max_conns`
en `DATABASE_URL`, pgxpool usa "the greater of 4 or runtime.NumCPU()" (documentación de
pgxpool). El plan no fija ese valor en ningún lado.

Como la espera máxima en el lock es (P − 1) × retención, que se cumpla "0 `503`" depende de
cuántas CPU tenga el host:

- con 2 vCPU, P = 4;
- con 32 vCPU y la retención de hoy, 31 × 90 ms ≈ 2,8 s: `503` seguros en una ráfaga.

## 3. Decisión 1: redefinir el target (pregunta 1)

Se separa la causa del efecto en dos targets. Uno sobre la **retención**: es lo que controla el
diseño y lo que DD-33 R-b quería acotar. Otro sobre la **ráfaga**: es lo que ve el usuario, si
recibe `503` o no.

| | Antes (§13, T-B905) | Después (decimotercera revisión) |
|---|---|---|
| Retención del lock | "Transacción de registro, desde `provision_tenant_role` hasta el `COMMIT`": p95 < 250 ms **medido con 10 simultáneos** | Desde el inicio de `provision_tenant_role` hasta el fin del `COMMIT`, **sin contención** (un registro por vez), con 10.000 roles: **p95 < 140 ms**. Al menos 100 muestras, en cada una de dos corridas |
| Ráfaga | "0 `503` con 10 concurrentes", una tanda | **0 `503`** con 10 simultáneos, **5 repeticiones** (50 muestras), con `pool_max_conns = 8` explícito |
| Con 50 | Se reportan los `503` | Se reportan los `503`, la latencia de punta a punta (incluida la espera por el pool) y la de `GET /me` durante la ráfaga |
| Tamaño del pool | Implícito (default de pgxpool) | DD-33 R-e: explícito en producción, con (P − 1) × retención p95 ≤ `lock_timeout` / 2 |

**De dónde sale 140 ms.** Con todo el pool ocupado por registros, el último espera
(P − 1) × retención. Para que esa espera no pase de la mitad del `lock_timeout` (2 s / 2 = 1 s)
con P = 8, la retención tiene que ser ≤ 1000 / 7 ≈ 143 ms; se redondea a 140. El factor 2 es el
margen para lo que el benchmark no ve: otro hardware, `autovacuum`, una reprovisión en curso.

**Por qué esto no es mover el arco después de ver el resultado:**

1. El target de retención es **más estricto** que el anterior leído como retención: 140 ms
   contra 250.
2. Sale de un parámetro que el diseño ya tenía (el `lock_timeout` de DD-33 R-a) y de P = 8, el
   valor con que se midió, que ahora queda fijado.
3. La retención **todavía no se midió**: el target y las reglas de decisión (§5) se fijan antes
   de T-B910.
4. Si la retención no cumple, la cláusula de ADR-005 se aplica tal como está escrita.

## 4. Decisión 2: T-B910, la medición que falta (pregunta 2)

Hace falta antes de decidir, y es una tarea para el desarrollador **sin cambios de diseño**. El
texto canónico va en `tasks.md`; se repite acá para que esta revisión se lea sola.

**T-B910 — Re-medición con 10.000 empresas: retención del lock, ráfagas y desglose** · R-2, R-3,
R-15, R-17, ADR-005, DD-33 (R-b, R-e), DD-34, INV-26 · decimotercera revisión

- **Alcance**
  - Solo código de test en `internal/testsupport/phase9bench/`, con build tag `bench` y fuera de
    `make check`.
  - Reemplaza a `TestPhase9Benchmark` de T-B905 y conserva sus mediciones. `registration_tx_10`
    y `registration_tx_50` se siguen reportando, sin target, para comparar.
  - No cambia el código de producción, `lock_timeout`, el pool de runtime, los targets ni el
    diseño. Si algo no se puede medir sin tocar producción, se reporta "no medido" y se sigue.
- **Entorno**
  - El de la corrida 1: imagen por digest, `server_version` 18.6 verificada, 10.000 empresas
    cargadas con `Register` real, hash precalculado salvo en login y signup HTTP.
  - Un pool propio del bench como `crm_app`, con `pool_max_conns=8` **explícito** en el DSN, para
    no depender de la cantidad de CPU.
  - Un tracer del bench en `ConnConfig.Tracer` que implementa `pgx.QueryTracer` y
    `pgxpool.AcquireTracer`. En pgx v5.11, `BEGIN` y `COMMIT` pasan por `Conn.Exec`, así que
    también se trazan; pgxpool usa el `AcquireTracer` si el tracer de la conexión lo implementa.
  - El log registra digest, `server_version`, Go, SO/arquitectura, CPU, memoria de Docker y
    `pool_max_conns`.
- **Instrumentación**
  - El bench pone un id de muestra en el contexto antes de llamar a `Register`. El tracer
    registra, por evento, el id de muestra, el PID del backend, una etiqueta y tiempos
    monotónicos.
  - **Etiquetas**: el nombre de sqlc (`-- name: X`); `set_role:crm_signup`,
    `set_role:crm_auth` y `set_role:tenant`, reconocidos por el texto de `setRole`; `begin`,
    `commit` y `acquire`.
  - **Intervalos por muestra**:

    | Intervalo | Qué mide |
    |---|---|
    | `pool_wait` | El `acquire` |
    | `pre_callback` | Del fin del `acquire` al fin de `set_role:crm_signup` |
    | `provision` | La sentencia `ProvisionTenantRole`; con contención incluye la espera por el lock |
    | **`hold`** | Del inicio de `ProvisionTenantRole` al fin del `commit`. Es la cota superior de la retención: incluye el `CREATE ROLE`, que va antes del `GRANT` que toma el lock |
    | `callback` | Como en la corrida 1 |
    | `end_to_end` | De la llamada a `Register` a su retorno |

  - **Autocontroles** (el test falla si no se cumplen):
    - toda sentencia de una muestra tiene una etiqueta conocida y el id de muestra;
    - cada muestra tiene exactamente un `begin`, un `ProvisionTenantRole` y un `commit`;
    - en M-2, los `hold` de muestras distintas no se solapan.
- **Mediciones**:

  | Id | Qué | Cómo | Se reporta |
  |---|---|---|---|
  | M-1 | Curva de escala | Durante la carga de las 10.000, por bloque de 1.000 registros | p50/p95 por bloque de `hold`, `provision`, `set_role:tenant`, `pre_callback` y `callback`; duración de cada bloque |
  | M-2 | Registro aislado con 10.000 roles | 100 `Register` secuenciales después de la carga, sin otra actividad | p50/p95/máx de cada intervalo y de cada sentencia etiquetada |
  | M-3 | Cuerpo de `provision_tenant_role` | 50 transacciones por el pool de superusuario: `SET LOCAL ROLE crm_provisioner`, y después `CREATE ROLE`, `GRANT crm_tenant … WITH INHERIT TRUE, SET FALSE` y `GRANT <rol> TO crm_app WITH INHERIT FALSE, SET TRUE`, con los mismos atributos que la función, cada uno medido; `ROLLBACK` | p50/p95 por sentencia. Es una aproximación: no pasa por plpgsql ni por `SECURITY DEFINER` |
  | M-4 | Primer uso en otra conexión después de un registro | 20 veces: el bench toma 7 conexiones del pool de medición con `Acquire` y corre un `Register` (que usa la 8.ª). En cada una de las 7 abre una transacción, mide dos veces seguidas el `setRole` de `crm_auth` con el mismo texto que `platform/db` (la primera y la "caliente"), hace `ROLLBACK` y la libera. Además, 20 `GET /api/v1/me` por el router real, cada uno inmediatamente después de un registro | p50/p95/máx de la primera y de la caliente (140 muestras cada una); p95 del `GET /me` posterior a un registro |
  | M-5 | Ráfagas | 10 y 50 `Register` simultáneos (barrera como en la corrida 1), **5 repeticiones** de cada una. Entre repeticiones se espera a que el pool no tenga conexiones tomadas. Durante cada ráfaga de 50, una goroutine hace `GET /api/v1/me` secuenciales por el router real, en el mismo pool, desde la barrera hasta que vuelve el último `Register` | Por tamaño de ráfaga: muestras, cantidad de `503` y p50/p95/máx de `end_to_end`, `pool_wait`, `provision`, `hold` y `callback`. Espera estimada en el lock = `provision` − p50 de `provision` en M-2. `GET /me` durante la ráfaga: muestras y p50/p95/máx |
  | M-6 | Lo de la corrida 1 | Mismo método que T-B905 | Los mismos ocho valores. Para `SET LOCAL ROLE`, además, las 10 muestras de calentamiento por separado; `set_role_retry_total` |

- **Corridas**
  - Dos completas, cada una con un contenedor nuevo; cada target se evalúa en las dos.
  - No se agregan corridas para desempatar ni se repite una buscando aprobar.
  - Duración estimada: entre 30 y 45 minutos de máquina en total.
- **Targets** (`plan.md` §13, decimotercera revisión). El test falla si alguno no se cumple en
  alguna de las dos corridas:

  | Id | Target |
  |---|---|
  | T-1 | `hold` de M-2: p95 < 140 ms |
  | T-2 | Ráfaga de 10 (M-5): 0 `503` en las 50 muestras |
  | T-3 | M-6, p95: `GET /me` < 50 ms, logo `304` < 50 ms, login < 400 ms, signup < 1000 ms |
  | — | `set_role_retry_total` = 0 (como en T-B905: si no, runbook de plan §12.3) |

- **Reglas que se reportan sin hacer fallar el test** (línea `BENCH rule <id> triggered=<bool>`):

  | Id | Condición | Consecuencia (§5) |
  |---|---|---|
  | D-a | Algún `503` con 50 simultáneos | Solo se reporta: es el comportamiento aceptado de DD-33 R-a |
  | D-b | `GET /me` durante la ráfaga de 50 con p95 > 1 s | Se le propone al usuario O-4 |
  | D-c | M-4: primer `setRole` después de un registro con p95 > 250 ms | Volver al arquitecto |
  | D-d | M-1: el p95 de `hold` del bloque 9.001–10.000 dividido el del bloque 4.001–5.000 da > 2,5 | Volver al arquitecto |

  **Por qué 2,5 en D-d**: si el costo crece linealmente con N, el cociente entre 10.000 y 5.000
  roles es ≤ 2 (menos, por la parte fija); si crece con el cuadrado, se acerca a 4. Un cociente
  > 2,5 dice que el margen de 10× sobre la escala supuesta (S-3: ~1.000 empresas) deja de ser
  confiable.
- **Reporte**
  - Va en la entrada de la Fase 9 de "Estado de la implementación": una tabla por medición y por
    corrida, y el veredicto por target y por regla.
  - El desarrollador no decide ni cambia el diseño.
  - Si T-1, T-2 o T-3 no se cumplen, o se dispara D-c o D-d, igual completa el checkpoint de la
    fase y lo reporta (§6).

## 5. Decisión 3: opciones y reglas de decisión (pregunta 3)

### Por qué no se elige ahora

Los datos no distinguen entre dos situaciones:

- la retención es de ~85 ms y el diseño cumple con margen a esta escala;
- la retención es mayor y crece con N.

Elegir el *pool* de roles o B con estos datos sería decidir por un artefacto de la medición.
Elegir mantener el diseño sería afirmar algo que nadie midió. T-B910 cuesta una corrida del arnés
que ya existe; cualquiera de las otras opciones cuesta una fase.

### Opciones, con su costo y su efecto en fases cerradas

| Opción | Qué cambia | Fases cerradas afectadas | Qué resuelve | Qué no resuelve, y qué cuesta |
|---|---|---|---|---|
| **O-1** Mantener ADR-005 con DD-33 (R-a a R-e) y los targets redefinidos | Documentos y el bench | Ninguna en código | Nada nuevo: acepta R-2, ahora con números medidos | Si H-1 es cierta, R-2 crece con N y hay que re-medir antes de las 5.000 empresas. Una ráfaga mayor que el pool sigue pudiendo dar `503` (aceptado en DD-33) |
| **O-2** *Pool* de roles creados de antemano (research R-04c) | Tabla de stock en `provisioning`; función que crea roles sin empresa; una `PeriodicTask` que crea K roles por transacción; `Register` reclama uno con `FOR UPDATE SKIP LOCKED` y usa su UUID como `tenants.id`; camino de respaldo con stock vacío (el actual, con DD-33); métricas de stock | Fase 1 (migraciones, función, tests de catálogo T-B107 a T-B109), Fase 3 (`Register`, T-B303, T-B305), Fase 9 (inventario, donde los roles sin empresa dejan de ser "sobrantes"; reprovisión) | Saca el `GRANT` y el lock del camino del registro. Elimina DD-34/R-17 en la práctica: la membresía ya es vieja en el primer uso | Redefine INV-14 (el rol existe antes que la empresa). Choca con ADR-008: el UUIDv7 llevaría la hora de creación del rol, no la del registro. R-2 sigue (el job invalida las cachés igual, aunque en lote). El respaldo mantiene vivo el camino actual. Más piezas para un equipo que está aprendiendo |
| **O-3** Alternativa B de ADR-005: rol único `crm_tenant` + `SET LOCAL app.tenant_id` | `app.current_tenant_id()` lee la variable, solo bajo `crm_tenant`. El cambio de rol de empresa en `setRole` pasa a ser `crm_tenant` + la variable. `Register` deja de aprovisionar. Se retiran `provision_tenant_role`, `crm_provisioner`, la reprovisión, el inventario de roles y el reintento de DD-34. Un ADR nuevo reemplaza a ADR-005 | Fases 0 a 3 (bootstrap, migraciones, `platform/db`, `Register`), Fase 8 (se vuelve a correr `Isolation`), Fase 9 (T-B906, T-B907 y el inventario quedan obsoletos) | DD-33, DD-34, R-2, R-3, R-15, R-17 y el riesgo de hosting de P-1 desaparecen de raíz | Revierte la decisión del usuario. Se pierden la identidad por empresa en `pg_stat_activity` y los límites por rol, que son el fundamento de ADR-005. Una fase entera |
| **O-4** Semáforo de registros en el proceso: capacidad 1, espera acotada a `lock_timeout`, después `503` con `Retry-After` | `tenant.Service.Register`, antes de `InSystemTx` | Fase 3 (`Register`, filas nuevas en T-B303) | Una ráfaga deja de retener conexiones del pool mientras espera el lock, así que los demás requests no esperan. Con una instancia, la espera en el lock queda en ≈ 0 | No baja la retención ni R-2. Con más de una instancia (S-1), el lock vuelve a ser la cola. Solo tiene sentido si se dispara D-b |
| Subir `lock_timeout` | Una constante | Fase 3 | Menos `503` | La espera sigue y retiene conexiones del pool más tiempo. **Descartada**, por la misma razón por la que research R-04c descartó subir `statement_timeout` |

### Reglas de decisión (fijadas antes de medir)

| Resultado de T-B910 | Decisión |
|---|---|
| T-1, T-2 y T-3 se cumplen en las dos corridas; D-c y D-d no se disparan | **O-1**. DD-33 registra los números. T-B910 se repite con 20.000 roles antes de que `tenants_total` llegue a 5.000 (§12.4). Si además se disparó D-b, se le propone O-4 al usuario como DD nueva dentro de A |
| T-1 o T-3 no se cumplen, en alguna corrida | Se aplica la cláusula de ADR-005 ("Recomendación del arquitecto", punto 2): el arquitecto escribe un ADR *Proposed* que reemplaza a ADR-005, con **O-3 recomendada** y O-2 como alternativa. Decide el usuario; fase nueva en `tasks.md`. 001 no sale a producción hasta implementarla |
| T-1 se cumple y T-2 no | Contradice el modelo de cola (7 × 140 ms < 2 s). Volver al arquitecto con M-5 antes de decidir nada |
| D-c o D-d se disparan | Volver al arquitecto: R-2 en el camino de lectura de todas las empresas (D-c) o crecimiento más que lineal (D-d). Lo más probable es que termine en la fila anterior: un ADR que reemplace a ADR-005 |
| Una corrida cumple y la otra no | Volver al arquitecto con las dos; no se agrega una tercera para desempatar |

**Por qué O-3 antes que O-2 en la rama de falla**: B elimina seis riesgos documentados con un
cambio en el lugar que el diseño reservó para eso (`app.current_tenant_id()` y `TxRunner`; ADR-005,
"Reversibilidad"). El *pool* de roles agrega piezas, redefine INV-14, conserva R-2 y su respaldo
mantiene vivo DD-33. Su única ventaja es conservar la decisión del usuario, y esa decisión es de
él.

## 6. Decisión 4: qué pasa con la Fase 9 (pregunta 4)

| Qué | Cuándo | Por qué |
|---|---|---|
| Prueba independiente final con `crm serve` real: `/readyz` con migrate down/up, dos ciclos de muestreo después de borrar y restaurar un rol, SIGTERM con un request en curso | Se puede correr **ya**, como señal temprana. La del checkpoint se corre sobre el HEAD final, después de B-2 y B-3, porque B-3 toca `serve.go` | No depende de T-B905: verifica T-B901 a T-B909 |
| Parte B (B-2, B-3) | Después de la aprobación | Cambian una regla del guard y agregan una costura en `cmd/crm` |
| T-B910 | Después de la aprobación | Los targets y las reglas se fijan antes de medir |
| Checkpoint de la Fase 9 | Con T-B910 reportada, **cualquiera sea su veredicto** | Ver abajo |
| Salida a producción de 001 | Solo con T-1, T-2 y T-3 cumplidos en las dos corridas y sin D-c ni D-d. Si no, con el ADR que reemplace a ADR-005 implementado (o con la decisión del usuario que lo evite) | ADR-005, consecuencia 1: "hay que medirlo … antes de salir" |

**Por qué el veredicto no bloquea el cierre de la fase**: T-B910 mide una propiedad del código de
las Fases 1 y 3, que ya está en `main`. El código de la Fase 9 (limpieza, `/readyz`, métricas,
reprovisión, apagado) no la cambia, así que bloquear el merge del PR #17 no protege nada y frena
el resto del trabajo.

- **Alternativa descartada**: dejar la fase abierta hasta que T-B910 pase. Mezcla un gate de
  release con el cierre de una fase y, en la rama de falla, la dejaría abierta durante toda la
  migración.
- **Trade-off aceptado**: si el resultado lleva a B, `main` tiene por un tiempo código que B
  vuelve obsoleto (reprovisión, inventario de roles). Se retira en la fase de B.

## 7. Supuestos de la parte A

| Id | Supuesto | Si es falso |
|---|---|---|
| S-a | La retención es de ~84–91 ms (modelo de cola FIFO del lock, con 8 en vuelo) | Es justamente lo que mide T-B910; ninguna decisión se apoya en este número |
| S-b | H-1: el costo dominante con miles de roles es reconstruir las listas de membresía después de cada cambio de `pg_auth_members`. No está verificado en el código de PostgreSQL; es consistente con el reporte de PostgreSQL 16 y con el arreglo de PostgreSQL 17 | El desglose de M-2 y M-3 lo dice. Si el costo está en otro lado (p. ej. `CREATE ROLE` o el `COMMIT`), las opciones no cambian, pero sí cuánto ayudaría O-2 |
| S-c | Docker Desktop en macOS agrega latencia por viaje de red (`GET /me`, con ~7 viajes, tarda 10 ms); producción será Linux | Los targets se evalúan en este entorno, que es conservador. Si la diferencia importa para la decisión, T-B910 se repite en un runner Linux |
| S-d | PostgreSQL 16+ le concede a `crm_provisioner` una membresía con `ADMIN` sobre cada rol que crea (cambios de `CREATEROLE`; ver "Fuentes" de research) | Solo cambia el detalle de H-1, no las reglas |

---

# Parte B — Pendientes de la revisión de código del PR #17

Los tres los aportó el `code-reviewer`; cada uno quedó verificado contra el código de la rama.

## B-1. API exportada que no figura en `plan.md` §11.1

Firmas reales:

| Paquete | Identificador | Firma | Comportamiento que conviene dejar escrito |
|---|---|---|---|
| `internal/app` | `PeriodicTasks` | `func PeriodicTasks(runner db.TxRunner, c clock.Clock, logger *slog.Logger) []outbox.PeriodicTask` | Único lugar que decide qué tareas corre el `Dispatcher`: `identity.Cleanup`, `outbox.Stats`, `tenant.RoleInventory` |
| `internal/app` | `LogServerVersion` | `func LogServerVersion(ctx context.Context, runner db.TxRunner, logger *slog.Logger)` | `InSystemTx(crm_auth)` con plazo `ReadinessTimeout`. Con éxito, `INFO` `database connected` con `server_version` (R-4). Si falla y no fue cancelación, `WARN` y el arranque sigue |
| `internal/app` | `ShutdownTimeout` | `const ShutdownTimeout = outbox.SendBudget + 5*time.Second` (25 s) | Presupuesto del apagado (T-B909, DD-35): `Serve` espera los requests y `crm serve` espera al worker, cada uno como mucho este plazo |
| `internal/identity` | `NewCleanup` | `func NewCleanup(runner db.TxRunner, logger *slog.Logger) *Cleanup` | `Name()` = `identity_cleanup`, `Every()` = 1 h; logger nil → `slog.Default()` |
| `internal/platform/outbox` | `NewStats` | `func NewStats(runner db.TxRunner, clock clock.Clock) *Stats` | `outbox_stats`, 1 min. **El constructor publica la instancia** como fuente de los gauges de `/debug/vars`: la última construida gana |
| `internal/tenant` | `NewRoleInventory` | `func NewRoleInventory(runner db.TxRunner) *RoleInventory` | `tenant_role_inventory`, 1 min. Publica la instancia igual que `NewStats` |
| `internal/tenant` | `ReprovisionLockKey` | `const ReprovisionLockKey int64 = 0x43524d525052` | Clave del *advisory lock* de sesión de la reprovisión (DD-33 R-d) |
| `internal/tenant` | `Reprovision` | `func (s *Service) Reprovision(ctx context.Context) (ReprovisionReport, error)` | **Exige que el `db.TxRunner` del `Service` implemente también `db.SessionLocker`**: lo comprueba con una aserción de tipo y, si no, devuelve un error enseguida |

**Decisión**: se documentan tal como están en §11.1; no cambia código. Dos rasgos quedan
aceptados a sabiendas:

- **Constructores que publican su instancia** (`NewStats`, `NewRoleInventory`). `expvar` es global
  del proceso y cada binario construye una sola instancia de cada uno.
  - *Alternativa descartada*: que `internal/app` llame a una función `Publish` explícita. Es más
    cableado sin ganancia con una instancia por proceso.
  - *Trade-off*: un test que construya dos instancias publica la segunda; queda escrito en §11.1
    para que nadie se sorprenda.
- **Aserción de tipo en `Reprovision`**. En Go, preguntar con una aserción si un valor "también
  sabe hacer X" es un idiom común cuando la capacidad es **opcional** (`io.WriterTo`). Acá la
  capacidad es **obligatoria** para ese método, y la dependencia no se ve en la firma: un idiom
  usado fuera de su caso.
  - *Alternativa*: `Reprovision(ctx, locker db.SessionLocker)`, o una opción de `NewService`.
  - *Por qué se acepta igual*: hay un solo llamador (`crm tenants reprovision-roles`), la falla es
    inmediata y clara en la primera ejecución, y T-B906 la cubre. Cambiarlo toca la firma y sus
    tests por un beneficio de legibilidad.
  - Queda escrito en §11.1. Si aparece un segundo llamador, se pasa a parámetro explícito.

## B-2. El guard de T-B801 se esquiva con valores de función

**Qué pasa.** Las reglas (1), (3) y (4) cuentan **llamadas** a `app.BuildAPIRouter`,
`app.NewRootHandler`, `app.NewServer`, `app.NewMetricsServer` y `app.Serve`. Si la función se
guarda en una variable y se la llama por ella, la llamada ya no se resuelve a la función
protegida. `mk := app.NewMetricsServer; mk(cfg)` y `serve := app.Serve; serve(…)` dan 0
violaciones. Verificado contra el texto de las reglas: ninguna habla de usos que no son llamadas.

**Decisión: regla (5) nueva, con la propuesta del revisor.** Cada aparición en `Info.Uses` del
`types.Func` de esas cinco funciones tiene que ser exactamente el `Fun` de un `ast.CallExpr`: por
la selección `app.X`, o por el identificador si el import es con punto, y sin paréntesis de por
medio. Cualquier otro uso es una violación: asignarla, pasarla como argumento, guardarla en un
campo o en un literal, `reflect`. `go app.Serve(…)` y `defer app.Serve(…)` siguen siendo llamadas
directas.

- **Por qué contar desde los usos y no agregar casos**: una lista de formas prohibidas se esquiva
  con la próxima que nadie listó. "Solo como llamada directa" cierra todas las formas de hacer
  pasar la función por un valor.
- **Mutaciones nuevas**:
  - (e) `mk := app.NewMetricsServer` y `mk(cfg)` → reglas (5) y (4);
  - (f) `serve := app.Serve` y `serve(serverCtx, srv, ln, logger)` → regla (5);
  - (g) `(app.Serve)(…)` → regla (5);
  - más las de la regla (4) ajustada en B-3.

## B-3. Falta un test de `runServe`, y la costura que necesita

**Qué falta.** Dos mutaciones de `cmd/crm/serve.go` sobreviven a `make check`:

- sacar la espera de `workerDone`, que contradice T-B908 y DD-35;
- sacar el `stopServers()` que sigue a cada `app.Serve`, que contradice T-B904: si un servidor
  falla, el otro y el worker se apagan.

La primera se puede provocar sin tocar el código: un SMTP de prueba que retiene la conversación,
configurado por `getenv`. La segunda, no. Hace falta que un `Serve` falle, y `runServe` crea sus
listeners adentro con `net.ListenConfig`.

**Decisión: una costura en `env`, la regla (4) ajustada y T-B911.**

- **La costura.** `env` (`cmd/crm/main.go`, que ya existe para que los tests reemplacen el mundo
  exterior) suma
  `listen func(ctx context.Context, network, address string) (net.Listener, error)`.
  - `run` lo inicializa con `(&net.ListenConfig{}).Listen`.
  - `runServe` escucha `HTTP_ADDR` y `METRICS_ADDR` solo con `e.listen`.
  - Un test llama a `runServe` con un `listen` que envuelve listeners reales y puede hacer fallar
    `Accept` cuando lo decide.
  - `net` no es `net/http`: la regla (2) no cambia.
- **Regla (4) ajustada.** El listener es una variable de un solo uso asignada desde una llamada a
  `e.listen` (el campo `listen` de un valor de tipo `env`), con la dirección igual que antes:
  directamente `cfg.HTTPAddr` o `cfg.MetricsAddr`. Además:
  - en los archivos no-test de `cmd/crm`, el campo `env.listen` aparece, fuera de esas dos
    llamadas, **una sola vez**: en el literal de `env` de `run`, con valor exactamente
    `(&net.ListenConfig{}).Listen` o `new(net.ListenConfig).Listen`;
  - ninguna otra asignación ni lectura del campo.

  Así el guard sigue demostrando que el binario escucha con un listener real, en la dirección
  configurada.
- **Alternativas descartadas**:
  - *Mover la orquestación (dos servidores, worker, cancelación cruzada) a `internal/app` y
    testearla ahí con fakes*: es lo más limpio en Go (un `main` delgado y la lógica en un paquete
    testeable), pero reescribe las reglas (3) y (4), que ya se rehicieron dos veces, en la última
    fase y por dos mutaciones. Queda como mejora si `runServe` vuelve a crecer.
  - *Una variable de paquete `var listen = …`*: es estado global mutable (impide `t.Parallel()`) y
    la regla (5) necesitaría una excepción.
  - *No testear la falla de un `Serve`*: deja sin red el comportamiento que T-B904 exige.

**T-B911 [T] — Tests de `runServe`: falla de un servidor y espera del worker** · T-B904, T-B908,
T-B909, DD-35, plan §9.4 · decimotercera revisión

- **Costura**: `env.listen` como arriba. Nada más cambia en `runServe`.
- **Red** (integración, en `cmd/crm`, contra el PostgreSQL del harness y un SMTP de prueba propio
  del test, sin `t.Parallel()`):

  | Caso | Esperado |
  |---|---|
  | El `listen` del test envuelve listeners reales. Con los dos servidores atendiendo, el `Accept` de `HTTP_ADDR` pasa a devolver un error permanente | `runServe` vuelve antes de `app.ShutdownTimeout` con un error que envuelve el del `Accept` (`errors.Is`). Después, `METRICS_ADDR` no acepta conexiones y el `Dispatcher` terminó |
  | Lo mismo con el `Accept` de `METRICS_ADDR` | Simétrico: `HTTP_ADDR` deja de aceptar, el worker terminó y `runServe` devuelve el error |
  | Se cancela el contexto (equivalente a SIGTERM) con un mensaje del outbox en envío, contra un SMTP de prueba que acepta la conexión y retiene la conversación hasta que el test lo libera (menos de los 5 s por etapa de DD-35) | `runServe` **no** vuelve mientras el SMTP retiene (el test lo verifica durante al menos 500 ms). Al liberarlo, vuelve con `nil` antes de `app.ShutdownTimeout`. El mensaje queda `sent` o `pending`, nunca a medio actualizar (T-B908) |
  | Se cancela el contexto sin trabajo en curso | `nil` |

- **Mutaciones** (se aplican, el caso falla y se restauran):
  - quitar el `stopServers()` que sigue al `app.Serve` de la API → falla la fila 1 (el test acota
    la espera; `runServe` no vuelve);
  - lo mismo con el de métricas → falla la fila 2;
  - quitar la espera de `workerDone` → falla la fila 3;
  - devolver `nil` en lugar de `serveErr` → fallan las filas 1 y 2.
- **Green**: las cuatro filas pasan. El guard de T-B801, con las reglas (4) y (5) nuevas, sigue
  en verde sobre el código real y falla con las mutaciones (e) a (i) (las de la (4) están en
  `tasks.md`). `make check` en verde.

## B-4. Invariantes y decisiones de la parte B

- Se preservan INV-22 (mux raíz) y DD-22. El guard queda más estricto, sin cambiar lo que
  garantiza.
- §11.1 no cambia ninguna firma existente: solo documenta las que faltaban.
- La costura `env.listen` es la única concesión a la testeabilidad en código de producción. La
  regla (4) la acota: un solo valor posible en producción.

---

## 8. Invariantes (toda la revisión)

- **Se preservan sin cambios**: INV-02, INV-03, INV-14 (registro atómico, incluido el rol), INV-22,
  INV-26 (`lock_timeout`, sin E/S de red; solo suma T-B910 a su columna de tests) e INV-27.
- **DD-33**: R-b se reformula (el objetivo pasa a ser la retención sin contención) y se agrega
  R-e (pool explícito). Ninguna de las dos cambia código: R-e es configuración de despliegue
  (`pool_max_conns` en `DATABASE_URL`).
- **ADR-005**: la decisión no cambia. Se agrega la nota (c), con el mismo formato que las notas
  del 2026-09-30.

## 9. Lo que tiene que aprobar el usuario

| # | Pregunta | Default propuesto |
|---|---|---|
| 1 | ¿Aceptás el diagnóstico (corrida 1 no concluyente) y los dos targets nuevos: retención sin contención con p95 < 140 ms y ráfaga de 10 con 0 `503`? | Sí |
| 2 | ¿DD-33 R-e: `pool_max_conns` explícito en producción, 8, con (P − 1) × retención ≤ 1 s? | Sí, 8 |
| 3 | ¿T-B910 con sus dos corridas y las reglas de decisión de §5, fijadas antes de medir? | Sí |
| 4 | ¿La Fase 9 se cierra con T-B910 reportada, cualquiera sea el veredicto, y la condición pasa a la salida a producción? (La alternativa es dejarla abierta hasta que T-B910 pase.) | Sí, cerrar |
| 5 | ¿Regla (5) del guard y regla (4) ajustada con la costura `env.listen`, más T-B911? (La alternativa es mover la orquestación de `runServe` a `internal/app`.) | Sí, costura en `env` |
| 6 | ¿Se documentan en §11.1 tal como están los constructores que publican su instancia y la aserción de `Reprovision`? (La alternativa es un parámetro explícito.) | Sí, documentar |

## 10. Cambios en los documentos (cuando se apruebe)

- **`plan.md`**:
  - encabezado y tabla de artefactos;
  - INV-26 (columna de tests);
  - DD-33: R-b, R-e nueva y trade-off;
  - §11.1: `internal/app`, `tenant.Reprovision` y constructores de `PeriodicTask`;
  - §12.3: fila "Registros con `503` en ráfaga";
  - §12.4: crecimiento;
  - §13: dos filas y el párrafo;
  - §15: R-2 y R-15;
  - §16: una fila modificada y tres nuevas;
  - §18: estado de la implementación y decimotercera tanda.
- **`tasks.md`**:
  - encabezado;
  - "Estado de la implementación";
  - T-B801: regla (4), regla (5) y mutaciones;
  - Fase 9: prueba independiente, nota en T-B905, T-B910 y T-B911 nuevas, checkpoint y condición
    de salida;
  - trazabilidad.
- **`research.md`**: encabezado, nota en R-04c, fuentes y supuesto 15.
- **`docs/adr/005-…`**: nota (c).

Ningún archivo del contrato ni `data-model.md` cambia.

## 11. Hallazgos de documentación

1. `plan.md` §18, "Estado de la implementación (2026-10-05)", dice que la Fase 6 está en curso:
   está desactualizado desde la Fase 7. Se actualiza en esta tanda.
2. T-B905 le puso el target a la cota superior de la retención medida con concurrencia (§2.1).
   Error de la tercera revisión, del arquitecto.
3. §13 hacía una cuenta que depende del tamaño del pool sin fijarlo (§2.4).
4. El bench de T-B905 descarta las 10 primeras muestras de `SET LOCAL ROLE` sin reportarlas. Es
   razonable como calentamiento, pero oculta el costo que R-2 anticipa; T-B910 las reporta aparte.
5. §16 no tenía una fila para la API exportada nueva ni para cómo arranca `crm serve`. Por eso las
   firmas de la Fase 9 (B-1) no llegaron a §11.1 sin que nada lo marcara.

## 12. Matriz de mantenimiento (agregado)

| Cuando cambies… | También tenés que actualizar… |
|---|---|
| `pool_max_conns` de producción, `lock_timeout` del registro o la cantidad de instancias | DD-33 R-a/R-e + §13 (la cuenta de la ráfaga) + T-B910 + T-B303 (si cambia `lock_timeout`) |
| La cantidad de empresas se acerca a 5.000 (`tenants_total`) | Repetir T-B910 con 20.000 roles; actualizar §13 y DD-33 con los números nuevos |
| Cómo arranca o se apaga `crm serve` (servidores, listeners, worker, `env`) | Reglas (1) a (5) de T-B801 en el mismo PR + T-B911 + §11.1 (`internal/app`) + §9.4 |
| Una función, tipo o constante exportada de `internal/` que usa otro paquete | §11.1 en el mismo PR |

## Fuentes

- Hilo "Slow GRANT ROLE on PostgreSQL 16 with thousands of ROLEs" (pgsql-hackers, marzo de 2024):
  - <https://www.postgresql.org/message-id/907785.1711121266%40sss.pgh.pa.us>
  - <https://www.postgresql.org/message-id/20240322163952.GA2347986%40nathanxps13>
- Commit d365ae7 de PostgreSQL 17, "Optimize roles_is_member_of() with a Bloom filter":
  <https://postgresql.org/message-id/E1rpCj9-005ouH-7L%40gemulon.postgresql.org>
- pgxpool, default de `pool_max_conns` ("the greater of 4 or runtime.NumCPU()"):
  <https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool>
- pgx v5.11.0 en el *module cache* del proyecto:
  - `tx.go`: `BeginTx` y `Commit` ejecutan por `Conn.Exec`;
  - `conn.go`: `Exec` llama a `QueryTracer`;
  - `pgxpool/tracer.go`: `AcquireTracer`;
  - `pgxpool/pool.go`: usa el `AcquireTracer` si `ConnConfig.Tracer` lo implementa.
