# Validación del atlas

Fecha local: 2026-10-14. Revisión de referencia única: `9e6fc311b81221ea051710dc5af08ad244475170` (origin/main al iniciar esta corrección).

## Contenido y enlaces

- 33 temas, 38 páginas con diagramas, 59 figuras, 17 recorridos y 40 HTML en total.
- Cada figura actual y las partes actuales de las vistas mixtas se contrastaron contra esa revisión; [registro por figura](REVISION.md).
- 120 URLs únicas de GitHub fijadas al mismo SHA: 106 rutas distintas comprobadas con `git cat-file -e <SHA>:<ruta>`; 22 anclas de línea verificadas contra la longitud del blob del mismo SHA. Ninguna ruta ausente ni rango inválido.
- Enlaces internos y anclas comprobados. La carpeta anterior de referencias locales se eliminó y no queda ningún enlace hacia ella.
- Los 15 permisos se contrastaron con authz.go de la revisión. Las siete tablas y su DDL mantienen los mismos blobs de migraciones; se agregan las referencias de privilegios 00001 y readiness 00009.
- IDs SVG sin duplicados y máscaras de etiquetas sin solapar nodos. La figura de CI se revisó visualmente y no tiene texto fuera de nodos o viewBox.

## Self-check del plugin y adaptación explícita

Se ejecutó self_check sobre las 38 páginas. El validador original rechaza todos los hipervínculos externos y exige el controlador en inglés exacto: reporta los enlaces GitHub en 38 páginas y la localización de mensajes en las 17 interactivas. Son dos requisitos explícitos del usuario que reemplazan esas restricciones del plugin; no se informa un pase del validador original sin cambios.

El [validador reproducible](validar.py) vuelve a ejecutar todas las comprobaciones del plugin con dos adaptaciones acotadas: permite solo los hipervínculos GitHub ya verificados y compara el controlador completo contra la plantilla con los literales de estado traducidos. No permite recursos remotos ni cambios adicionales en el JavaScript. Resultado: **38/38**, sin incidencias estructurales, de seguridad, geometría o movimiento detectadas por self_check.

Desde la raíz del repositorio:

```sh
python3 docs/diagramas/validar.py --repo . --self-check /ruta/diagram-design/scripts/self_check.py
git diff --check
```

Sin --self-check, validar.py sigue comprobando enlaces internos, SHA, rutas Git, rangos de línea, recursos inline y conteos; no ejecuta las pruebas geométricas del plugin.

## Navegador

- Las 38 páginas se revisaron de nuevo en escritorio: sin texto fuera del viewBox o de los nodos, ni desbordamiento horizontal de página.
- Las 38 páginas se recorrieron a 390 × 844 px: sin desbordamiento de página; cada figura tiene su propio scroll horizontal.
- En las 17 secuencias: Siguiente, Inicio, Fin, flecha derecha, Reproducir y Pausar comprobados en escritorio. Fin deja visibles todos los elementos y los anuncios de paso/pausa están en español. A 390 px se repitieron Siguiente y Fin en las 17.
- Cobertura móvil: 33 filas y sin desbordamiento. Navegación entre índice, temas y detalles comprobada.
- Sin JavaScript, impresión y movimiento reducido tienen comprobación estructural del plugin; no se emularon esas preferencias en el navegador. Los literales de sus anuncios también están traducidos.

## Alcance y checks remotos

No se editaron plan, tasks, ui, ADRs ni contrato. Los enlaces del README general, visión, modelo de dominio e índice ADR se conservan. No se ejecutaron despliegues, migraciones ni operaciones financieras.

make check y make vuln son checks requeridos del PR; su resultado para el SHA final se consulta en GitHub y se informa al cerrar esta revisión. No se sustituye su resultado con la validación documental. El ruleset de main se contrastó por la API de GitHub.

La reproducción automática continua sigue pendiente por indicación del usuario. Las vistas Especificado conservan su estado; las máquinas de estados de negocio siguen siendo representativas.
