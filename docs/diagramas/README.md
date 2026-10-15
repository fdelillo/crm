# Atlas de diagramas del CRM

[Abrir índice](index.html) · [Cobertura](cobertura.html) · [Estilo](ESTILO.md) · [Revisión de contenido](REVISION.md) · [Validación](VALIDACION.md) · [Manifest](manifest.json)

33 temas, 38 páginas con diagramas, 59 figuras y 17 recorridos interactivos. Los 33 temas del catálogo inicial están cubiertos.

## Organización por área

- Arquitectura y seguridad: contexto, módulos, petición, aislamiento, permisos y datos.
- Flujos de identidad y servicios: registro, sesión, usuarios, recuperación, correo y logos.
- Desarrollo y operación: backend, contratos, CI, pruebas, runbook, observabilidad, trazabilidad y frontend previsto.
- Negocio previsto: ciclo comercial, presupuestos, proyectos, finanzas, monedas, cheques, importación y reportes.

## Abrir y navegar

Abrir `index.html` en un navegador o servir la carpeta como sitio estático. Cada HTML contiene CSS y SVG inline; no descarga fuentes tipográficas ni ejecuta servicios externos. GitHub muestra HTML como código. **Las referencias de código y diseño necesitan conexión**: son enlaces permanentes al repositorio público, con SHA y anclas de línea para los fragmentos.

- Controles: Anterior, Siguiente, Reproducir, Pausar y Reiniciar.
- Teclado dentro de un recorrido: flechas, Inicio, Fin, Espacio y R.
- Más de una secuencia por tema: el tema enlaza páginas de detalle.
- Anuncios de estado para lectores de pantalla en español rioplatense.
- Sin JavaScript, con movimiento reducido o al imprimir: figura completa.
- Captura estática: agregar `?motion=static` a una página con recorrido.

## Revisión y alcance

Una sola revisión de referencia: `9e6fc311b81221ea051710dc5af08ad244475170`, SHA de `origin/main` al revisar el PR. Se incorporan ADR-026 (CI, toolchain, hallazgos y checks requeridos) y DD-F31 (relectura de `/me` tras el reenvío). [Revisión por figura](REVISION.md).

- **Actual**: respaldado por código y migraciones de esa revisión.
- **Mixto**: combina piezas existentes y previstas, identificadas en figuras y notas.
- **Especificado**: diseño conceptual; no implica implementación ni aprobación de todos los detalles.

Las notas explican guardas, agrupaciones y omisiones. Las máquinas de estados de negocio son representativas; no sustituyen futuros planes ni una matriz exhaustiva de transiciones. El ruleset de main es configuración externa: se contrastó por la API de GitHub además de su documentación en ADR-026; no está versionado en el SHA.

El hosting de producción, su implementación operativa y las specs en borrador siguen pendientes del proyecto. No se ejecutan despliegues, migraciones ni operaciones de dinero.

## Mantenimiento

Cuando cambien arquitectura, contrato, permisos, migraciones o reglas financieras, actualizar figuras afectadas y todos sus enlaces a una nueva revisión única; registrar esa revisión en índice, manifest e informe de validación. Mantener el estilo de `ESTILO.md` y comprobar enlaces, accesibilidad, geometría y controles en escritorio y celular.

No servir estos HTML dentro de la SPA: sus estilos y scripts inline tienen una política de contenido distinta. Usar un sitio documental separado o archivos locales. [Pendientes de interacción](PENDIENTES.md).
