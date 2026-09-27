# ADR-011: Almacenamiento de archivos compatible con S3, subida a través del backend

**Status**: Accepted (S3 compatible y subida vía backend: decisión del usuario; librería y reglas: propuesta del arquitecto)
**Fecha**: 2026-09-27
**Origen**: spec 001 (logo de la empresa); lo reutilizan 004 (adjuntos) y 005 (PDF)

## Contexto

La constitución fija almacenamiento compatible con S3 para logos, adjuntos y PDFs. En 001 se sube
el logo de la empresa, que luego se imprime en los PDF de presupuestos (005). Un archivo subido
por un usuario es entrada no confiable (tipo falso, SVG con scripts, imágenes gigantes).

## Decisión

- Puerto `objectstore.ObjectStorage` (`Put`, `Get`, `Delete`) con un adaptador implementado con
  **`github.com/minio/minio-go/v7`**, que habla con cualquier servicio S3 compatible. En
  desarrollo, **MinIO**.
- **Bucket privado**. Los archivos se sirven a través del backend (endpoint autenticado), no con
  URLs públicas.
- **Subida vía backend** (`multipart/form-data`) con validación antes de guardar: tipo por
  contenido (*magic bytes*), tamaño máximo con `http.MaxBytesReader`, y para imágenes
  dimensiones máximas leídas del encabezado (`image.DecodeConfig`) sin decodificar la imagen.
- Claves con prefijo de empresa: `tenants/{tenant_id}/{tipo}/{uuidv7}.{ext}`. La clave se genera
  en el servidor; nunca se usa un nombre de archivo del usuario.
- Orden de operaciones: **subir antes del `COMMIT`** que guarda la referencia, y **borrar el
  objeto anterior después** del `COMMIT`. La base nunca apunta a un objeto inexistente; en el peor
  caso queda un objeto huérfano (aceptado).
- Reglas del logo (001): PNG o JPEG, ≤ 2 MB, ≤ 2000×2000 px; se sirve con `Content-Type` fijo y
  `X-Content-Type-Options: nosniff`.

## Fundamento

- Validar antes de guardar impide que un archivo peligroso llegue al bucket.
- Servir por el backend mantiene el control de acceso por empresa (RLS sobre la referencia) y
  evita configurar CORS o políticas públicas en el bucket.
- El prefijo por empresa facilita auditoría, borrado y cuotas futuras.
- minio-go tiene una API pequeña y directa, adecuada para un equipo que está aprendiendo.

## Alternativas consideradas

- **`aws-sdk-go-v2`**: oficial y completo, pero mucho más grande y con configuración adicional
  (*path-style*) para MinIO. Alternativa válida si se necesitan funciones específicas de AWS.
- **URLs prefirmadas para subida directa**: el backend no transfiere bytes, pero la validación
  ocurre después de que el archivo ya está en el bucket y requiere CORS. Para los adjuntos de
  15 MB de 004 se puede reevaluar en su plan.
- **Guardar archivos en PostgreSQL (`bytea`)**: infla la base y los backups.
- **Aceptar SVG**: puede contener scripts; descartado.

## Consecuencias

- Ganás: validación fuerte, bucket privado, un solo camino de acceso controlado.
- Aceptás: el backend transfiere los bytes (irrelevante para logos de ≤ 2 MB; a revisar para
  adjuntos grandes); objetos huérfanos ocasionales, que una limpieza futura puede detectar
  comparando claves con referencias.
