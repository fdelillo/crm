# Validación del atlas completo

Fecha: 2026-10-14. Revisión fuente: `c7692bd7b7a0d8e07e18d3c9ec1738b169dc032f`.

## Resultado

- 33 temas principales: cobertura del catálogo inicial documentada en `cobertura.html` y `COBERTURA.md`.
- 38 páginas con diagramas, 59 figuras y 17 recorridos interactivos; cinco páginas de detalle enlazadas desde sus temas.
- Las 38 páginas pasan `diagram-design/scripts/self_check.py --offline`: SVG accesible, contenido dentro del viewBox, recursos autocontenidos y controlador canónico de movimiento.
- Enlaces locales de 143 HTML comprobados, incluidas las copias de fuentes y la cobertura. Sin referencias remotas.
- Las 15 filas de permisos coinciden con authz. El esquema físico conserva las siete tablas reales; los modelos futuros están identificados como especificados.
- Máscaras de etiquetas sin solapar nodos. La comprobación de rutas no encontró cruces nuevos ni strokes compartidos; los cuatro cruces previos conservan sus puentes.

## Revisión en navegador

- Entrega 01: 10 páginas en escritorio y móvil; controles de petición/aislamiento, reinicio, reproducción y pausa comprobados.
- Entrega 02: 10 páginas nuevas en escritorio y a 390 px; diez secuencias llegan a fin por teclado con todos los elementos visibles.
- Entrega 03: 18 páginas nuevas en escritorio y a 390 px, sin texto fuera del SVG/nodos ni desbordamiento horizontal de página. Las cinco secuencias nuevas llegan a fin por teclado.
- Cada figura mantiene su ancho legible y scroll horizontal independiente en celular.
- Corregida y comprobada el área pulsable de los títulos del índice que se parten en varias líneas.
- Cobertura HTML con 33 filas. Se verificaron enlaces entre índice, temas y detalles.

Los modos sin JavaScript, impresión y movimiento reducido se comprobaron estructuralmente en HTML/CSS con el validador del plugin; no se emularon en el navegador. No se ejecutó `make check`: esta entrega cambia documentación estática y no ejecuta código de negocio, despliegues, migraciones ni movimientos financieros.

Las specs en borrador y los detalles Proposed conservan ese estado. Hosting, backups operativos y RPO/RTO no se presentan como ya resueltos. Las máquinas de estados de negocio son representativas y no reemplazan los planes futuros.

La reproducción automática continua sigue pendiente por indicación del usuario.
