# Módulo Bienvenidos y Banner

**Estado:** integrado en Cortex Go (Go + páginas web incluidas en el ejecutable).
**Diseño de referencia:** módulo anterior “Bienvendos y Banner”. Se adaptaron su estructura visual, hojas de estilo, animaciones, dos fondos de nebulosa y tipografías; no se copiaron `.env`, `settings.json`, sesiones, perfiles ni `uploads`.

## Acceso

Con Cortex Go ejecutándose, abre:

- Panel de control: `http://127.0.0.1:17845/modules/bienvenidos/admin`
- Starting Soon para OBS: `http://127.0.0.1:17845/modules/bienvenidos/starting-soon`
- Banner Social para OBS: `http://127.0.0.1:17845/modules/bienvenidos/banner`

En OBS crea una fuente de navegador por cada overlay y usa **720 × 1280**. Si OBS está en otra computadora, configura como URL el host de Cortex en el tailnet. El panel remoto usa el mismo origen de Tailscale Serve. OBS solo puede conectar por loopback a OBS WebSocket de la computadora que ejecuta Cortex.

## Controles recuperados

- Contador regresivo con iniciar, pausar, reiniciar, +1 minuto y ±10 segundos; duración inicial de hasta 99:59. El contador usa una hora de fin almacenada por Cortex y se resincroniza con el panel y ambas fuentes del navegador.
- Textos de insignia, encabezado, título y subtítulo; colores primario, secundario, glow y fondo; familias Orbitron, Rajdhani, Share Tech Mono, Inter, Outfit y Montserrat, servidas localmente.
- Partículas, velocidad, glow y fondos de color, imagen o video. Permite subir logo, imagen/video de fondo y música, más control de volumen.
- Lista de redes flotantes con plataforma, usuario y visibilidad.
- Banner inferior rotativo con plataformas Instagram, Facebook, TikTok, Telegram y Web; etiqueta, usuario, visibilidad, intervalo, orden aleatorio, barajar y pausa al pasar el puntero.
- Vista previa dentro del panel.
- Conexión de lectura para comprobar OBS y acciones de cambio de escena manual o automáticas.

## OBS

Cortex usa `ws://127.0.0.1:4455` por defecto. Para otra dirección se puede definir `CORTEX_OBS_WS`, pero el host debe ser loopback; no se aceptan direcciones de red. Si OBS requiere contraseña, Cortex la lee de `CORTEX_OBS_PASSWORD`. La contraseña no aparece en el panel, la configuración del módulo ni los logs.

La casilla **Auto-transición cuando el contador llegue a 00:00** empieza desactivada. Para usarla, guarda el nombre exacto de una escena local. Al alcanzar cero, el servidor intenta cambiar una sola vez a esa escena. El botón **Cambiar a Escena Ahora** ejecuta la acción bajo demanda. La prueba de conexión solo verifica la sesión WebSocket.

## Persistencia y archivos

La migración SQLite v2 añade tablas para configuración, runtime y archivos. La configuración y el estado del contador sobreviven a reinicios; también los archivos multimedia porque se guardan como BLOB en la base activa y entran en sus respaldos. Un respaldo puede crecer según los medios cargados.

Tipos y límites máximos:

| Uso | Formatos | Tamaño máximo |
|---|---|---:|
| Logo | PNG, JPEG, WebP, GIF | 8 MiB |
| Fondo imagen | PNG, JPEG, WebP | 20 MiB |
| Fondo video | MP4, WebM | 30 MiB |
| Música | MP3, WAV, OGG | 16 MiB |

El servidor valida extensión y firma del archivo antes de guardarlo. Al sustituir un medio, elimina el archivo anterior si ya no se utiliza. Los comentarios de Facebook siguen separados del módulo y no se incorporan a sus páginas ni logs.

## Rutas de API

El navegador usa el proxy del panel local; el servidor interno mantiene las rutas:

- `GET /api/modules/welcome/state`
- `GET` y `PUT /api/modules/welcome/settings`
- `POST /api/modules/welcome/timer` con `start`, `pause`, `reset` o `sync`
- `POST /api/modules/welcome/obs/test` y `/obs/transition`
- `POST /api/modules/welcome/upload/{logo|bg-image|bg-video|bg-music}`
- `GET /api/modules/welcome/assets/{id}` con soporte Range para medios

Las páginas y scripts se incluyen en el ejecutable; no dependen de CDN, GSAP, Font Awesome, Google Fonts remoto ni servidor Node separado.

## Validación pendiente en OBS

La integración compila en Go y se debe aceptar en la computadora que tenga Cortex Go y OBS activos: abrir ambas URLs, confirmar encuadre, cargar medios de prueba, mover el contador y comprobar la escena manualmente con OBS WebSocket local. La transición automática debe probarse solo después de elegir una escena de prueba y activar deliberadamente su casilla.
