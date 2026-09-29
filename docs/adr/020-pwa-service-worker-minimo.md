# ADR-020: PWA instalable con manifest y un service worker mínimo, sin funcionamiento offline

**Status**: Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`ui.md` §20); constitución, principio VII
**Revisión 2026-09-29**: nombre de la app e íconos confirmados como provisorios por el usuario (P-F1:
"CRM", ícono genérico).

## Contexto

La constitución pide distribuir como PWA instalable y **no** requiere offline en el MVP. Chrome
dejó de exigir un service worker con *fetch handler* para instalar desde el menú (108 en móvil,
112 en escritorio), pero la sugerencia automática de instalación sigue apoyándose en él, y sin un
SW propio el navegador muestra su página genérica de error al abrir la app sin conexión. Un
service worker que cachea la app o la API introduce el problema más caro de las PWAs: versiones
viejas servidas desde caché y datos de una sesión visibles para otra en un dispositivo compartido.

## Decisión

- **Manifest** (`public/manifest.webmanifest`): `id`, `name` y `short_name` = "CRM" (provisorios,
  P-F1), `lang: es-AR`, `start_url: /`, `scope: /`, `display: standalone`, colores de los tokens,
  íconos genéricos 192, 512 y 512 `maskable`; `apple-touch-icon` en `index.html`.
- **Service worker escrito a mano** (`public/sw.js`, sin librerías):
  - Atiende **solo** navegaciones (`request.mode === 'navigate'`) que no empiecen con `/api/`: red
    primero; si la red falla, responde `offline.html` desde su caché.
  - Cualquier otro request (API, assets, logo, manifest) **no se intercepta**.
  - Única caché: `crm-offline-v<N>` con `offline.html` (autocontenida, sin scripts). `activate`
    borra las versiones anteriores; `skipWaiting()` y `clients.claim()`.
  - Se registra en producción, después del evento `load`, con `scope: /`.
- **Sin** botón "Instalar" propio en el MVP.
- Cuando una spec pida offline, se reemplaza este ADR (probablemente por Workbox/vite-plugin-pwa y
  un diseño de sincronización).

## Fundamento

- Cumple "instalable" en Android (menú y sugerencia automática) e iOS ("Agregar a inicio" solo
  necesita el manifest y el ícono).
- No cachear la app ni la API elimina de raíz los problemas de versiones mezcladas y de datos de
  otra sesión: el SW no puede servir nada viejo porque no guarda nada de la app.
- Una página "Sin conexión" propia, en español, es mejor que el error genérico del navegador en
  obra con señal intermitente.
- ~30 líneas legibles cuestan menos de entender que la configuración de Workbox.

## Alternativas consideradas

- **vite-plugin-pwa (Workbox) con precache del *app shell***: arranque más rápido en visitas
  repetidas, pero precachea la app (hay que diseñar el aviso de "versión nueva" y su recarga) y
  agrega una dependencia con muchos conceptos; es lo indicado cuando haya offline.
- **Sin service worker**: instalable desde el menú de Chrome, pero sin sugerencia automática y con
  la página de error genérica sin conexión.
- **Cachear respuestas de `/api` (stale-while-revalidate)**: mostraría datos viejos y datos de
  otro usuario tras cerrar sesión; prohibido por NFR-F09 e INV-F09 (y la API responde `no-store`,
  DD-28 del plan).

## Consecuencias

- Ganás: instalable, página propia sin conexión, cero riesgo de versiones o datos viejos servidos
  por el SW.
- Aceptás: sin mejoras de velocidad por precache (la caché HTTP con `immutable` cubre las visitas
  repetidas); cambiar `offline.html` exige subir el número de caché; el SW se prueba en E2E
  (no en jsdom); cambiar el nombre o el ícono definitivos es editar el manifest y los íconos.
