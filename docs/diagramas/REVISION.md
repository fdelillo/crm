# Revisión de contenido por figura

Referencia única: `9e6fc311b81221ea051710dc5af08ad244475170` (origin/main al iniciar esta corrección). Fecha local: 2026-10-14.

Se contrastaron las figuras actuales y las partes actuales de las vistas mixtas con los objetos de esa revisión, antes de construir sus enlaces. Las 102 fuentes citadas inicialmente existen allí: 93 conservan el mismo blob Git que en la revisión anterior y nueve cambiaron (workflows CI, Makefile, README, índice ADR, go.mod, plan, tasks, ui y tools.mod). Se revisaron esos cambios y se incorporaron ADR-026, DD-F31 y sus nuevas fuentes. La comparación de objetos respalda que el código de identidad, aislamiento, outbox, logos y el DDL no cambió; los errores de explicación detectados se corrigieron igualmente.

La revisión semántica de las figuras se resume abajo. Actual se refiere a código/migraciones del SHA; Mixto identifica las partes implementadas y las previstas. Especificado sigue siendo diseño, sin ampliar su estado de aprobación. No se ejecutaron operaciones de negocio para esta revisión documental.

El ruleset de main se verificó con `gh api repos/fdelillo/crm/rules/branches/main`: PR requerido, make check y make vuln requeridos, reglas deletion y non_fast_forward. Esta configuración está fuera de Git y se observó durante esta revisión; la decisión está documentada en [ADR-026](https://github.com/fdelillo/crm/blob/9e6fc311b81221ea051710dc5af08ad244475170/docs/adr/026-vulnerabilidades-dependencias-govulncheck.md#L165-L170).

| Página / figura | Estado de la vista | Contraste y resultado |
|---|---|---|
| [01-contexto · Contexto del sistema](01-contexto.html) | Mixto | Se mantienen actores, límite CRM y servicios SMTP/S3; se distingue negocio previsto del backend actual. |
| [02-arquitectura · Arquitectura general](02-arquitectura.html) | Mixto | serve conserva API y worker, PostgreSQL/S3/SMTP; root conserva SPAUnavailableHandler 503 y SPA prevista. |
| [03-modulos · Mapa de módulos](03-modulos.html) | Mixto | tenant, identity, authz e industrytemplate existen; NoopSeeder no implementa configuración completa. |
| [04-fronteras · Dependencias y fronteras](04-fronteras.html) | Actual | Se contrastan fronteras de imports y depguard; app compone, tenant consume identidad y ambos usan platform. |
| [05-peticion · Recorrido de una petición](05-peticion.html) | Actual | PATCH repite authz.Can con el Principal y luego LockTenant, actualización y auditoría; DD-44 excluye releer actor/sesión en escrituras de empresa. |
| [06-aislamiento · Aislamiento entre empresas](06-aislamiento.html) | Actual | TxRunner, SET LOCAL ROLE, RLS forzada y reintento 42501 previo al callback permanecen iguales en esta revisión. |
| [07-permisos · Permisos compartidos](07-permisos.html) | Actual | 15 permisos y 6 del operador coinciden con authz.go; se agregan DD-48, DD-44 y la excepción GET /users. |
| [07-permisos · Permisos de administración](07-permisos.html) | Actual | 15 permisos y 6 del operador coinciden con authz.go; se agregan DD-48, DD-44 y la excepción GET /users. |
| [08-dominio · Modelo comercial](08-dominio.html) | Especificado | Modelo conceptual de negocio, sin afirmar tablas físicas ni implementación. |
| [08-dominio · Los tres libros financieros](08-dominio.html) | Especificado | Modelo conceptual de negocio, sin afirmar tablas físicas ni implementación. |
| [09-esquema · Empresa y usuarios](09-esquema.html) | Actual | Las siete tablas y sus DDL coinciden con las migraciones de referencia; se citan privilegios de 00001 y excepción de readiness de 00009. |
| [09-esquema · Sesiones y usuario](09-esquema.html) | Actual | Las siete tablas y sus DDL coinciden con las migraciones de referencia; se citan privilegios de 00001 y excepción de readiness de 00009. |
| [09-esquema · Tokens de un solo uso](09-esquema.html) | Actual | Las siete tablas y sus DDL coinciden con las migraciones de referencia; se citan privilegios de 00001 y excepción de readiness de 00009. |
| [09-esquema · Outbox por empresa](09-esquema.html) | Actual | Las siete tablas y sus DDL coinciden con las migraciones de referencia; se citan privilegios de 00001 y excepción de readiness de 00009. |
| [09-esquema · Auditoría y actor](09-esquema.html) | Actual | Las siete tablas y sus DDL coinciden con las migraciones de referencia; se citan privilegios de 00001 y excepción de readiness de 00009. |
| [09-esquema · Contadores de login](09-esquema.html) | Actual | Las siete tablas y sus DDL coinciden con las migraciones de referencia; se citan privilegios de 00001 y excepción de readiness de 00009. |
| [10-entorno-local · Entorno local de desarrollo](10-entorno-local.html) | Mixto | Compose, loopback, TLS local y SPA ausente contrastados; se precisa toolchain go1.27.2; Vite sigue previsto. |
| [11-registro · Registro de empresa](11-registro.html) | Actual | Registro mantiene cupos IP/global, aprovisionamiento y atomicidad de usuario/token/outbox/auditoría. |
| [12-autenticacion · Ingreso y creación de sesión](12-autenticacion.html) | Actual | Cupo IP antes del body (20, 1/3 s), orden hasher/conexión, bloqueo y vigencia de sesión; limpiar throttle requiere activo con credencial válida. |
| [12-autenticacion · Bloqueo por intentos](12-autenticacion.html) | Actual | Cupo IP antes del body (20, 1/3 s), orden hasher/conexión, bloqueo y vigencia de sesión; limpiar throttle requiere activo con credencial válida. |
| [12-autenticacion · Vigencia de una sesión](12-autenticacion.html) | Actual | Cupo IP antes del body (20, 1/3 s), orden hasher/conexión, bloqueo y vigencia de sesión; limpiar throttle requiere activo con credencial válida. |
| [13-usuarios-aceptacion · Aceptar la invitación](13-usuarios-aceptacion.html) | Actual | Cupos preview/accept compartidos; invitar/reinvitar/cambiar rol/desactivar/reactivar: lock empresa → requireActor → lock objetivo; GET /users no revalida. |
| [13-usuarios · Emitir una invitación](13-usuarios.html) | Actual | Cupos preview/accept compartidos; invitar/reinvitar/cambiar rol/desactivar/reactivar: lock empresa → requireActor → lock objetivo; GET /users no revalida. |
| [13-usuarios · Estados del usuario](13-usuarios.html) | Actual | Cupos preview/accept compartidos; invitar/reinvitar/cambiar rol/desactivar/reactivar: lock empresa → requireActor → lock objetivo; GET /users no revalida. |
| [14-recuperacion-reset · Confirmar una contraseña nueva](14-recuperacion-reset.html) | Actual | Reset: cupo IP 429 y email 202 silencioso. Reenvío: 429 antes del servicio; 202 sin emitir/revocar para verificado o inactivo; emisión solo para activo no verificado. |
| [14-recuperacion-verificacion · Confirmar el email](14-recuperacion-verificacion.html) | Actual | Reset: cupo IP 429 y email 202 silencioso. Reenvío: 429 antes del servicio; 202 sin emitir/revocar para verificado o inactivo; emisión solo para activo no verificado. |
| [14-recuperacion · Solicitar recuperación](14-recuperacion.html) | Actual | Reset: cupo IP 429 y email 202 silencioso. Reenvío: 429 antes del servicio; 202 sin emitir/revocar para verificado o inactivo; emisión solo para activo no verificado. |
| [14-recuperacion · Estados de los tokens](14-recuperacion.html) | Actual | Reset: cupo IP 429 y email 202 silencioso. Reenvío: 429 antes del servicio; 202 sin emitir/revocar para verificado o inactivo; emisión solo para activo no verificado. |
| [15-outbox · Entregar un correo pendiente](15-outbox.html) | Actual | Dispatcher, claim en crm_worker, entrega bajo tenant, clasificación SMTP, intentos y limpieza de payload sin cambios. |
| [15-outbox · Estados del outbox](15-outbox.html) | Actual | Dispatcher, claim en crm_worker, entrega bajo tenant, clasificación SMTP, intentos y limpieza de payload sin cambios. |
| [16-archivos-lectura · Consultar el logo](16-archivos-lectura.html) | Actual | Logo privado, autorización, locks, rollback seguro y reconciliación con lock tras COMMIT incierto sin cambios. |
| [16-archivos · Subir o reemplazar el logo](16-archivos.html) | Actual | Logo privado, autorización, locks, rollback seguro y reconciliación con lock tras COMMIT incierto sin cambios. |
| [16-archivos · Reconciliar un COMMIT incierto](16-archivos.html) | Actual | Logo privado, autorización, locks, rollback seguro y reconciliación con lock tras COMMIT incierto sin cambios. |
| [17-backend · Estructura interna del backend](17-backend.html) | Actual | Estructura handler/servicio/store y db.Tx/TxRunner contrastada; ningún cambio de backend en #23/#24. |
| [18-contratos · Contrato de la API](18-contratos.html) | Mixto | Contrato canónico 001, validación HTTP y generación SQL contrastados; generación TS sigue prevista. |
| [18-contratos · Generación del acceso SQL](18-contratos.html) | Mixto | Contrato canónico 001, validación HTTP y generación SQL contrastados; generación TS sigue prevista. |
| [19-ci · CI actual del backend](19-ci.html) | Mixto | Dos workflows, triggers, cron UTC, privilegios mínimos, toolchain exacto, guard y falla cerrada contrastados con ADR-026/Makefile/workflows; ruleset consultado por API. |
| [19-ci · Build completo previsto](19-ci.html) | Mixto | Dos workflows, triggers, cron UTC, privilegios mínimos, toolchain exacto, guard y falla cerrada contrastados con ADR-026/Makefile/workflows; ruleset consultado por API. |
| [20-pruebas · Garantías por nivel de prueba](20-pruebas.html) | Mixto | Capas de pruebas actuales contrastadas con harness/contrato/invariantes; navegador frontend aún previsto. |
| [21-operacion · Preparación y arranque](21-operacion.html) | Mixto | Arranque, readiness 1 s, apagado y restauración conservados; se agrega runbook de vulncheck y aceptación con vencimiento máximo 30 días. |
| [21-operacion · Apagado ordenado](21-operacion.html) | Mixto | Arranque, readiness 1 s, apagado y restauración conservados; se agrega runbook de vulncheck y aceptación con vencimiento máximo 30 días. |
| [21-operacion · Restauración en otro clúster](21-operacion.html) | Mixto | Arranque, readiness 1 s, apagado y restauración conservados; se agrega runbook de vulncheck y aceptación con vencimiento máximo 30 días. |
| [22-observabilidad · Logs, métricas y auditoría](22-observabilidad.html) | Actual | Logs/expvar/auditoría contrastados; se precisan 00001/00007 y el alcance de 00009. |
| [23-trazabilidad · Desarrollo y trazabilidad](23-trazabilidad.html) | Actual | Se conserva trazabilidad SDD y se agregan make check + make vuln como checks requeridos. |
| [24-frontend · Arquitectura del frontend](24-frontend.html) | Especificado | Frontend sigue especificado; se incorpora DD-F31/INV-F17: releer /me tras 202, ignorar staleTime y resolver sent/already_verified/unknown. |
| [24-frontend · Actualizar datos y refrescar la vista](24-frontend.html) | Especificado | Frontend sigue especificado; se incorpora DD-F31/INV-F17: releer /me tras 202, ignorar staleTime y resolver sent/already_verified/unknown. |
| [25-navegacion · Mapa de navegación y acceso](25-navegacion.html) | Especificado | Navegación sigue especificada; S-05 con email confirmado no ofrece Reenviar. |
| [26-comercial · Circuito comercial completo](26-comercial.html) | Especificado | Circuito comercial conceptual contrastado con specs en borrador; no se atribuye implementación. |
| [27-presupuestos · Estados y versiones del presupuesto](27-presupuestos.html) | Especificado | Estados, aprobación y deuda conceptual contrastados; no se atribuyen endpoints ni tablas. |
| [27-presupuestos · Efectos de aprobar un presupuesto](27-presupuestos.html) | Especificado | Estados, aprobación y deuda conceptual contrastados; no se atribuyen endpoints ni tablas. |
| [28-proyectos · Etapas de un proyecto](28-proyectos.html) | Especificado | Etapas configurables y cierre con advertencia conforme a spec 004; sin enum implementado inventado. |
| [29-finanzas-compra · Registrar una compra y su costo](29-finanzas-compra.html) | Especificado | Compra genera deuda/costo; cobro cancela deuda y mueve caja atómicamente; diseño conceptual de specs 006/007. |
| [29-finanzas · Registrar un cobro de cliente](29-finanzas.html) | Especificado | Compra genera deuda/costo; cobro cancela deuda y mueve caja atómicamente; diseño conceptual de specs 006/007. |
| [30-monedas-anulacion · Cobro entre monedas](30-monedas-anulacion.html) | Especificado | Dos importes explícitos y anulación con trazabilidad conforme a specs; sin tasa persistida inventada. |
| [30-monedas-anulacion · Anular una operación completa](30-monedas-anulacion.html) | Especificado | Dos importes explícitos y anulación con trazabilidad conforme a specs; sin tasa persistida inventada. |
| [31-cheques · Cheques de terceros](31-cheques.html) | Especificado | Estados representativos de cheques y rechazo compensatorio, sin transición automática depósito/cobro inventada. |
| [31-cheques · Cheques propios](31-cheques.html) | Especificado | Estados representativos de cheques y rechazo compensatorio, sin transición automática depósito/cobro inventada. |
| [32-importacion · Importación y conciliación](32-importacion.html) | Especificado | Importación, duplicados y propuesta de conciliación conforme a spec 008; confirmación manual. |
| [33-reportes · Linaje de reportes y margen](33-reportes.html) | Especificado | Linaje conceptual de reportes; exclusión de anulados/transferencias y previsión de cheques sin duplicarlos. |

Las referencias precisas se encuentran al pie de cada página y en manifest.json. Sus rutas, SHA y rangos de líneas se verifican con `validar.py`, usando `git cat-file -e` y la longitud del blob en ese mismo SHA. No se descarga ni se genera una copia de las fuentes.
