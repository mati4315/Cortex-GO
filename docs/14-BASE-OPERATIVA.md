# Estado de la base operativa

**Fecha:** 13 de noviembre de 2026 (Australia/Brisbane)
**Resultado:** núcleo y panel PWA publicados y desplegados en el equipo disponible. La base local existente se respaldó y migró; la PC habitual permanece apagada. La aceptación de Facebook real sigue pendiente porque requiere autenticación manual.

## Decisiones confirmadas

- La instancia activa se ejecuta en el equipo disponible y el panel remoto se limita al tailnet de Tailscale.
- Mantener una sola instancia escribiendo la base SQLite. Cuando la PC habitual esté disponible, cualquier traslado debe usar un respaldo verificado y detener primero el host actual.
- Los nombres y textos de Facebook se guardan en la base activa hasta que el usuario los borre manualmente.
- Borrar comentarios del historial elimina las filas de la base activa. Los respaldos son copias independientes: pueden conservar comentarios borrados, no están cifrados y deben eliminarse manualmente cuando corresponda.
- No se importan datos, perfiles, sesiones ni código del Cortex anterior. SocialStream Ninja queda excluido; las otras capacidades siguen en el plan de módulos.

## Implementado en el repositorio

### Arranque y recuperación

El ejecutable arranca un lanzador oculto que inicia y supervisa el servidor local. Si el proceso termina con error, el lanzador registra el motivo y hace hasta cinco reintentos con espera limitada. Un cierre normal desde el panel cierra ambos procesos. El panel permite habilitar o deshabilitar un acceso directo en la carpeta Inicio del usuario de Windows. El ajuste se aplica al próximo inicio de sesión, no cierra la instancia actual y no instala un servicio.

### API y datos

El panel escucha en `127.0.0.1:17845`; el servidor interno también usa loopback. La versión 1 crea el registro general `events` y migra transaccionalmente los eventos sintéticos anteriores. Cada evento conserva origen, cuenta, directo, ID externo cuando está disponible, autor, texto, hora original, hora de captura y clave de deduplicación. El índice evita repetir un evento dentro de su origen.

El panel permite consultar el historial, borrar una fila o todo el historial activo y ver el estado de las dos cuentas sin exponer nombres/textos en métricas o logs técnicos. `POST /api/events` acepta capturas; `POST /api/feeds/{id}/status` actualiza las métricas de la captura; las rutas `DELETE /api/events` y `DELETE /api/events/{id}` borran filas de la base activa.

La copia local valida SQLite y su esquema antes de restaurar; conserva una copia previa de la base actual y acepta respaldos compatibles con el esquema anterior. El panel enumera las copias generadas por Cortex y permite borrar una o todas con confirmación, para retirar comentarios borrados de las copias locales. Los archivos se guardan bajo `data\backups` asociado al directorio de datos. No hay cifrado ni sincronización privada implementados; copias exportadas a otro lugar se administran por separado.

### Panel PWA

El ejecutable sirve el manifiesto, los iconos y el service worker. La caché incluye solo la carcasa estática; no guarda API, estado ni comentarios. En HTTPS un navegador compatible ofrece instalación como aplicación. El panel conserva la conexión al backend del host principal. No hay login de Cortex: Serve se limita al tailnet, así que hay que restringir el acceso de Tailscale a los dispositivos de confianza.

### Facebook

`experiments/facebook-browser-poc` abre dos perfiles externos aislados de Edge o Brave, con sesiones en `%LOCALAPPDATA%`. El operador inicia sesión, resuelve 2FA e identifica visualmente cada cuenta. El observador establece una línea base, detecta comentarios nuevos y los envía al API local; el panel muestra estado y métricas por cuenta. El prototipo no imprime nombres ni textos en la consola.

Es una prueba externa, no la integración final en una ventana Cortex. No comprueba identidad, no automatiza la navegación a Live Producer y no incluye CEF. No se inició ni autenticó en Facebook durante este despliegue.

### OBS y otros módulos

OBS sigue en modo de solo lectura. No se cambian escenas ni se ejecutan acciones. Spotify, Rulo, sorteos y los demás módulos aún no están integrados.

## Despliegue y comprobaciones

El núcleo y el prototipo Facebook compilaron después de los últimos cambios. Las pruebas de la base pasaron durante la preparación; luego se acotó la lista de rutas permitidas al publicar. La instancia anterior tenía dos eventos sintéticos. Se generó un respaldo antes de actualizarla; la migración de SQLite conserva esos eventos en el registro general. Se comprobó el panel local y el acceso por Tailscale, además del estado del supervisor y del backend. No se autenticó Facebook ni se inició el prototipo, porque las cuentas requieren intervención manual.

## Pendiente de validación en los equipos y cuentas reales

1. Confirmar la estabilidad prolongada del host activo y que la PWA se instala/abre correctamente en los navegadores previstos.
2. Cuando la PC habitual esté encendida, decidir si se traslada la instancia. Verificar el respaldo, detener el host actual y arrancar solo una instancia sobre la base elegida.
3. Autenticar manualmente las dos cuentas; comprobar identidad, 2FA, aislamiento, persistencia de sesión, detección de comentarios nuevos y deduplicación. No ejecutar simultáneamente el Cortex anterior sobre las mismas cuentas.
4. Probar restauración con datos ficticios y decidir cómo cifrar los respaldos.
5. Continuar con la integración de OBS, Spotify, Rulo y demás módulos según prioridad; OBS permanece de solo lectura.

La base ya está publicada y desplegada para continuar el proyecto. La aceptación con sesiones reales y el eventual traslado a la PC habitual quedan pendientes. El Cortex anterior permanece sin cambios.
