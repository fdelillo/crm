# ADR-011: Almacenamiento de archivos compatible con S3, subida a través del backend

**Status**: Accepted (S3 compatible y subida vía backend: decisión del usuario; librería y reglas: propuesta del arquitecto)
**Fecha**: 2026-09-27
**Origen**: spec 001 (logo de la empresa); lo reutilizan 004 (adjuntos) y 005 (PDF)

> **Nota 2026-09-29 (detalle; no cambia la decisión)**, por el hallazgo H-2: los archivos de una
> empresa que sirve el backend usan `Cache-Control: private, no-cache` + `ETag` propio de cada
> objeto (en el logo, el UUIDv7 de su clave), y responden `304 Not Modified` ante un
> `If-None-Match` que coincide **sin leer el objeto de S3** (DD-23 del plan de 001). Nunca se
> usa `max-age` sobre una URL que es la misma para todas las empresas (`/api/v1/tenant/logo`):
> un caché con vida fija podría mostrar el logo de otra empresa en un navegador compartido. El
> cliente puede agregar un parámetro `v` para forzar una URL nueva tras un cambio; el servidor
> lo ignora. El resto de la API lleva `Cache-Control: no-store` (DD-28). No se crea un ADR nuevo
> porque el almacenamiento, el bucket privado y la subida vía backend no cambian: se fija la
> política de caché de la respuesta.

> **Nota 2026-09-29 (b) (detalle; precisa las reglas del logo; no cambia la decisión)**, por el
> hallazgo H-11 y la confirmación del usuario de que el navegador vuelve a codificar todo JPEG:
>
> 1. **Límite exacto** (decisión del usuario): el "≤ 2 MB" de las reglas del logo es **2 MiB =
>    2 097 152 bytes**, medidos sobre el contenido del archivo (la parte `file`), no sobre el
>    cuerpo `multipart/form-data`. El cuerpo completo tiene su propio límite, **2 162 688 bytes**
>    (archivo + 64 KiB para *boundary*, encabezados y nombre), aplicado con `http.MaxBytesReader`;
>    se acepta exactamente una parte `file` y se lee con un tope de `2 097 152 + 1` bytes (DD-31 del
>    plan de 001).
> 2. **Metadatos**: el servidor guarda los bytes validados **sin modificarlos**; no quita EXIF ni
>    aplica la orientación. La SPA vuelve a codificar todo JPEG (y así quita la ubicación), pero la
>    validación del backend no depende de eso: un cliente que no sea la SPA puede subir un JPEG con
>    EXIF (riesgo aceptado R-13 del plan de 001). Las specs que reusan este ADR (004, 005) definen
>    sus propios límites de archivo y de cuerpo del mismo modo.
>
> No se crea un ADR nuevo porque el tipo de almacenamiento, el bucket privado, la subida vía
> backend y la validación antes de guardar no cambian: se fija el valor exacto de una regla que ya
> estaba y cómo se mide.

> **Nota 2026-10-01 (detalle; qué imagen de MinIO se usa en desarrollo y tests; no cambia la
> decisión)**, por el hallazgo M10 de la revisión del PR fdelillo/crm#8 (DD-36 y research R-30 del
> plan de 001; **Accepted** 2026-10-01):
>
> 1. **Situación de MinIO** (verificada con búsqueda web el 2026-10-01): desde octubre de 2025 el
>    proyecto dejó de publicar binarios e imágenes de la edición comunitaria (solo código fuente);
>    el repositorio `minio/minio` está archivado desde abril de 2026, con
>    `RELEASE.2025-10-15T17-29-55Z` como última versión (incluye el arreglo de CVE-2025-62506);
>    `minio/minio` desapareció de Docker Hub en septiembre de 2026 y **`quay.io/minio/minio`
>    responde `401` a todo pull anónimo desde el 2026-09-24/25**, para todos los tags y digests. El
>    fallo de descarga del entorno de Codex no fue del entorno.
> 2. **Imagen elegida para `compose.yaml` y para los tests de integración, la misma en ambos**:
>    `ghcr.io/coollabsio/minio:RELEASE.2025-10-15T17-29-55Z`, fijada además por el digest de su
>    índice multiplataforma (`…:RELEASE.2025-10-15T17-29-55Z@sha256:<digest>`). Es una
>    recompilación de la última versión oficial desde su código fuente, con el `docker-entrypoint.sh`
>    de MinIO (mismo comando `server /data` y mismas variables `MINIO_ROOT_*`), pública y sin login.
>    Se descartan `bitnamilegacy/minio` (congelada en una versión anterior al arreglo de seguridad y
>    con otra estructura de arranque) y `cgr.dev/chainguard/minio` (mantenida y parcheada, pero su
>    nivel gratuito solo publica `latest` y no promete que un digest viejo siga disponible), que
>    queda como **respaldo**.
> 3. Producción no usa ninguna de estas imágenes: el servicio S3 compatible de producción se elige
>    con el hosting (P-1 del plan de 001). Como MinIO ya no tiene mantenimiento upstream, si la
>    imagen elegida deja de estar disponible o el cliente `minio-go` deja de ser compatible con ella,
>    se reabre la elección del servidor S3 **de desarrollo** (research R-30: Chainguard, una copia
>    propia en GHCR, o SeaweedFS/Garage); la decisión de este ADR (S3 compatible, bucket privado,
>    subida vía backend, `minio-go`) no depende de eso.

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
