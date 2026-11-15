# Estado de la base operativa

**Fecha:** 14 de noviembre de 2026 (Australia/Brisbane)
**Resultado:** instancia activa actualizada y reiniciada con SQLite v18, respaldo previo validado y supervisor/backend operativos. La conversación IA live queda apagada y no lista hasta configurar el límite monetario externo; autenticación real de Facebook y aceptación en OBS siguen pendientes.

## Decisiones confirmadas

- La instancia activa se ejecuta en el equipo disponible y el panel remoto se limita al tailnet de Tailscale.
- Mantener una sola instancia escribiendo la base SQLite. Cuando la PC habitual esté disponible, cualquier traslado debe usar un respaldo verificado y detener primero el host actual.
- Los nombres y textos de Facebook se guardan en la base activa hasta que el usuario los borre manualmente.
- Borrar comentarios del historial elimina las filas de la base activa. Los respaldos son copias independientes: pueden conservar comentarios borrados, no están cifrados y deben eliminarse manualmente cuando corresponda.
- No se importaron `.env`, perfiles, sesiones, configuraciones ni datos del Cortex anterior. Bienvenidos y Banner, Luces, Clima y Tiempo e Historial de Facebook están integrados en Go; Spotify, Rulo, sorteos y otros módulos siguen en el plan.

## Implementado en el repositorio

### Arranque y recuperación

El ejecutable arranca un lanzador oculto que inicia y supervisa el servidor local. Si el proceso termina con error, el lanzador registra el motivo y hace hasta cinco reintentos con espera limitada. Un cierre normal desde el panel cierra ambos procesos. El panel permite habilitar o deshabilitar un acceso directo en la carpeta Inicio del usuario de Windows. El ajuste se aplica al próximo inicio de sesión, no cierra la instancia actual y no instala un servicio.

### API y datos

El panel escucha en `127.0.0.1:17845`; el servidor interno también usa loopback. La versión 1 crea el registro general `events` y migra transaccionalmente los eventos sintéticos anteriores. La versión 2 crea almacenamiento SQLite de módulos y archivos; la versión 3 añade chats guardados; la versión 4 añade índices para consultas por fecha y transmisión. Cada evento conserva origen, cuenta, directo, ID externo cuando está disponible, autor, texto, hora original, hora de captura y clave de deduplicación. El índice evita repetir un evento dentro de su origen.

El panel permite consultar el historial, buscar por autor o texto, filtrar por cuenta/fecha/ID de transmisión, guardar chats por programa, exportar CSV para Excel, borrar una fila o todo el historial activo y ver el estado de las dos cuentas sin exponer nombres/textos en métricas o logs técnicos. Si llegan comentarios mientras se consulta una página anterior, un aviso permite cargar los más recientes sin interrumpir la lectura. `POST /api/events` acepta capturas; `POST /api/feeds/{id}/status` actualiza las métricas de la captura; las rutas `DELETE /api/events` y `DELETE /api/events/{id}` borran filas de la base activa.

La copia local valida SQLite y su esquema antes de restaurar; conserva una copia previa de la base actual y acepta respaldos compatibles con el esquema anterior. El panel enumera las copias generadas por Cortex y permite borrar una o todas con confirmación, para retirar comentarios borrados de las copias locales. Los archivos se guardan bajo `data\backups` asociado al directorio de datos. No hay cifrado ni sincronización privada implementados; copias exportadas a otro lugar se administran por separado.

### Panel PWA

El ejecutable sirve el manifiesto, los iconos y el service worker. La caché incluye solo la carcasa estática; no guarda API, estado ni comentarios. En HTTPS un navegador compatible ofrece instalación como aplicación. El panel conserva la conexión al backend del host principal. No hay login de Cortex: Serve se limita al tailnet, así que hay que restringir el acceso de Tailscale a los dispositivos de confianza.

La portada incluye **Estado del directo**: supervisor, servidor, SQLite, OBS, cada captura de Facebook y los módulos integrados. Muestra el estado observado, detalles de conexión y la última actividad disponible; las capturas aún no iniciadas o los problemas con servicios externos quedan marcados para revisión, sin declarar el directo listo por una mera respuesta HTTP.

### Facebook

`experiments/facebook-browser-poc` abre dos perfiles externos aislados de Edge o Brave, con sesiones en `%LOCALAPPDATA%`. El operador inicia sesión, resuelve 2FA e identifica visualmente cada cuenta. El observador establece una línea base, detecta comentarios nuevos y los envía al API local; el panel muestra estado y métricas por cuenta. El prototipo no imprime nombres ni textos en la consola.

Es una prueba externa, no la integración final en una ventana Cortex. No comprueba identidad, no automatiza la navegación a Live Producer y no incluye CEF. No se inició ni autenticó en Facebook durante este despliegue.

### OBS y otros módulos

El panel central consulta OBS sin enviar órdenes. Bienvenidos y Banner incluye una acción limitada para cambiar a una escena local: se ejecuta al solicitarla manualmente o al llegar el contador a cero solo si la opción automática quedó activada. El cambio automático empieza apagado. Rulo base/laboratorio y Spotify parcial ya están integrados. Lotería/Cdelu, Cuaderno Vivo, estadísticas públicas y la presentación final de Rulo siguen pendientes.

### Bienvenidos y Banner

El módulo está integrado en Go y usa el esquema SQLite v2. El panel, overlay vertical y banner social se sirven en el mismo origen de Cortex. Su estado de configuración, temporizador y archivos multimedia se incluyen en los respaldos SQLite. Consulta [la guía del módulo](15-MODULO-BIENVENIDOS-Y-BANNER.md) para las URLs, controles, límites de archivos y requisitos OBS.

## Despliegue y comprobaciones

El núcleo se compiló en el único ejecutable principal `cortex-go.exe`, se reinició y reportó estado activo. Antes de actualizar se creó `data\backups\CortexGo-20261113-062727-000000000-000.sqlite`; la migración conservó los dos eventos sintéticos existentes. Las comprobaciones manuales confirmaron respuesta HTTP 200 para el panel de historial y sus recursos, paginación y filtro por transmisión, consulta por fecha, campo de aviso de nuevos eventos, descarga CSV con BOM UTF-8 y esquema SQLite v4. También se ejecutaron `gofmt`, `node --check` para el JavaScript del historial y `git diff --check`. No se ejecutaron pruebas automatizadas. No se autenticó Facebook ni se comprobó una escena OBS real, porque la PC habitual está apagada.

El 13 de noviembre de 2026 se actualizó el overlay para quitar de la lista normal un comentario al destacarlo y se añadió **Más opciones** al panel central, con valores guardados en el navegador, URL OBS copiable y vista previa larga breve sin tarjeta duplicada. Antes del reinicio se creó `data\backups\CortexGo-20261113-104144-000000000-000.sqlite`. La compilación se hizo en `cortex-go.exe`; tras reiniciar se comprobó `healthz` y API 200, estado backend activo/SQLite conectada/esquema v4, las rutas del panel y overlay, y la presencia de los recursos y controles nuevos. Se abrió el panel en Brave, se recargó la pestaña que seguía mostrando la versión anterior y se confirmó visualmente que el botón abre el formulario. OBS no estaba conectado, así que queda pendiente aceptar la composición en una escena real.

En la misma fecha se agregó altura editable para el área de texto del chat normal (`chatHeight`) y para la tarjeta destacada (`featuredHeight`), aplicable también a la fuente independiente de destacados. La nueva compilación de `cortex-go.exe` se desplegó con respaldo previo `data\backups\CortexGo-20261113-114122-000000000-000.sqlite`. Se verificó de nuevo healthz/API, backend activo y SQLite v4 conectada; panel, controles de altura, generador de URL, scripts y estilos devolvieron HTTP 200 y los parámetros quedaron presentes. La escena real de OBS sigue pendiente de comprobación.

### Despliegue Rulo v18 — 14 de noviembre de 2026

Se creó el respaldo `data\backups\CortexGo-20261114-085750-000000000-000.sqlite` (212.992 bytes) mediante el endpoint administrado de Cortex antes del cierre ordenado. El acceso de inicio automático sigue apuntando a `F:\Cortex-GO\cortex-go.exe` con `--no-browser` y el directorio de trabajo esperado; se reinició esa misma ruta. Tras el arranque, `/healthz` respondió 200; supervisor/backend reportaron `running`, SQLite `connected`, esquema v18 y dos eventos. La portada, el panel Rulo y el historial Facebook respondieron HTTP 200; el panel incluye la evaluación live. El endpoint de evaluación rechazó la solicitud mientras la configuración live no está lista. Se confirmó `liveConversationEnabled=false` y `liveConversationReady=false`. No se autenticó Facebook, no se conectó Spotify ni se validó OBS. No se ejecutaron pruebas automatizadas.

## Pendiente de validación en los equipos y cuentas reales

1. Confirmar la estabilidad prolongada del host activo y que la PWA se instala/abre correctamente en los navegadores previstos.
2. Cuando la PC habitual esté encendida, decidir si se traslada la instancia. Verificar el respaldo, detener el host actual y arrancar solo una instancia sobre la base elegida.
3. Autenticar manualmente las dos cuentas; comprobar identidad, 2FA, aislamiento, persistencia de sesión, detección de comentarios nuevos y deduplicación. No ejecutar simultáneamente el Cortex anterior sobre las mismas cuentas.
4. Probar restauración con datos ficticios y decidir cómo cifrar los respaldos.
5. Validar manualmente el overlay y la fuente de navegador en la PC que ejecuta OBS; la notebook puede abrir el panel por Tailscale, pero no tiene OBS local.
6. Continuar con lotería/Cdelu, Cuaderno Vivo, estadísticas públicas y presentación; Spotify requiere aceptación con una cuenta real.

La base está desplegada y activa en el equipo disponible para continuar el proyecto. La aceptación con sesiones reales y el eventual traslado a la PC habitual quedan pendientes. El Cortex anterior permanece sin cambios.
