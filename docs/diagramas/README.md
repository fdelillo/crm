# Atlas de diagramas del CRM

[Abrir índice](index.html) · [Cobertura](cobertura.html) · [Estilo](ESTILO.md) · [Validación](VALIDACION.md) · [Manifest](manifest.json)

Tres entregas: 33 temas, 38 páginas con diagramas, 59 figuras y 17 recorridos interactivos. Los 33 temas del catálogo inicial están cubiertos.

1. Arquitectura, módulos, seguridad y datos: 10 temas / 17 figuras.
2. Registro, autenticación, usuarios, recuperación, correo y logos: 6 temas / 16 figuras.
3. Estructura backend, contratos, CI, pruebas, operación, observabilidad, trazabilidad, frontend y negocio previsto: 17 temas / 26 figuras.

## Abrir y navegar

Abrir `index.html` en un navegador o servir la carpeta como sitio estático. Cada HTML contiene CSS y SVG inline; no descarga fuentes ni recursos. GitHub muestra HTML como código. Las copias de fuentes también funcionan sin conexión.

- Controles: Anterior, Siguiente, Reproducir, Pausar y Reiniciar.
- Teclado dentro de un recorrido: flechas, Inicio, Fin, Espacio y R.
- Más de una secuencia por tema: el tema enlaza páginas de detalle, cada una con un controlador canónico.
- Sin JavaScript, con movimiento reducido o al imprimir: figura completa.
- Captura estática: agregar `?motion=static` a una página con recorrido.

## Estado y alcance

- **Actual**: respaldado por código y migraciones de la revisión fija del manifest.
- **Mixto**: combina piezas existentes y previstas, identificadas en figuras y notas.
- **Especificado**: diseño conceptual; no implica implementación ni aprobación de todos los detalles.

Los diagramas resumen: las notas explican guardas, agrupaciones y omisiones, y enlazan copias de sus fuentes. Las máquinas de estados de negocio son representativas; no sustituyen los futuros planes ni una matriz exhaustiva de transiciones.

El hosting de producción, su implementación operativa y las specs en borrador siguen pendientes del proyecto. Esta entrega completa la documentación propuesta, sin ejecutar despliegues, migraciones ni operaciones de dinero.

## Mantenimiento

Cuando cambien arquitectura, contrato, permisos, migraciones o reglas financieras, actualizar figuras afectadas, fuentes y manifest. Mantener el estilo de `ESTILO.md` y comprobar enlaces, accesibilidad, geometría y controles en escritorio y celular.

No servir estos HTML dentro de la SPA: sus estilos y scripts inline tienen una política de contenido distinta. Usar un sitio documental separado o archivos locales. [Pendientes de interacción](PENDIENTES.md).
