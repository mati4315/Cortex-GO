# Módulo Historial de Facebook

**Estado:** historial, overlay y una consulta agregada administrativa integrados en Cortex Go. La captura real todavía depende de iniciar sesión manualmente y abrir la vista de comentarios en los perfiles de Facebook; la composición final sigue pendiente de aceptación en una escena real de OBS. Las estadísticas públicas contestadas por Rulo no están habilitadas.

## Análisis de la referencia

Se revisó `Rulo/rulo-chat-historial.html` del proyecto anterior. La pantalla usa una lista de comentarios de lectura rápida, cabecera plegable, ajuste de tamaño de texto, tema claro/oscuro, panel de opciones, búsqueda, filtro por origen, estado de conexión, contador, borrado individual, limpieza confirmada, respaldo y actualización en vivo. También incluye respuestas del bot, archivos por número de programa, imágenes de perfil, acciones para destacar o repetir respuestas, controles de Spotify/OBS y métricas de audiencia. En los paquetes de referencia disponibles no se encontró una implementación ejecutable de esas métricas; la consulta nueva se limita a conteos sobre las tablas compartidas y no se presenta como un port exacto de consultas heredadas.

El detector actual entrega autor, texto, cuenta, origen, ID de comentario, transmisión y marcas de tiempo. No produce respuestas del bot, avatares ni datos de espectadores, y Cortex Go todavía no tiene los endpoints del Chat Bot Spotify. La pantalla conserva las funciones de historial que encajan con los datos existentes. No muestra controles que aparenten ejecutar acciones de respuesta, Spotify u OBS sin sus servicios correspondientes.

## Rutas

- `http://127.0.0.1:17845/rulo-chat-historial.html`
- `http://127.0.0.1:17845/modules/facebook/history`
- `http://127.0.0.1:17845/rulo-chat-overlay.html` (fuente de navegador OBS)
- `http://127.0.0.1:17845/modules/facebook/overlay`
- `http://127.0.0.1:17845/rulo-featured.html` (fuente de comentarios destacados para OBS)
- `http://127.0.0.1:17845/modules/facebook/featured`
- Mascota local extraída del overlay anterior: `GET /modules/facebook/assets/kangaroo.svg`
- API paginada: `GET /api/events?limit=50&offset=0&feedId=a&q=texto&streamId=123&from=...&to=...`
- Estadísticas agregadas de solo lectura: `GET /api/events/statistics?days=30&feedId=a` (`days` de 1 a 366; `feedId` opcional `a|b`).
- Comentarios en vivo para el overlay: `GET /api/modules/facebook/events?type=overlay&clientId=...` (SSE)
- Comentario seleccionado: `POST /api/modules/facebook/featured` con `{"eventId":123}` (actual) o `{"eventId":123,"archiveId":5}` (chat guardado); fuente SSE `GET /api/modules/facebook/featured/events?type=featured&clientId=...`
- Aviso de comentarios nuevos: `GET /api/events?limit=1&offset=0&afterId=123`
- Exportación filtrada: `GET /api/events/export.csv?feedId=a&q=texto&streamId=123&from=...&to=...`
- Chats guardados: `GET/POST /api/event-archives` y `GET/DELETE /api/event-archives/{id}`

La portada enlaza al historial completo y al overlay OBS. Las rutas sirven recursos embebidos desde `cmd/cortex-go/web/facebook/`; no requieren Node ni una base de datos adicional.

## Funciones

- Lista comentarios más recientes primero y pagina el registro SQLite completo, sin el límite visual de 100 elementos del panel central.
- Busca en autor, mensaje, origen y transmisión; filtra Cuenta 1 o Cuenta 2.
- Filtra por fecha local del navegador y por ID exacto de transmisión; puede combinarse con la cuenta y búsqueda de texto.
- Actualiza automáticamente cada cinco segundos. Si estás leyendo comentarios anteriores o más abajo en la lista, muestra un aviso con el número de comentarios nuevos y conserva tu posición.
- Muestra el estado observado del capturador, hora original del comentario, hora de recepción, cuenta y transmisión.
- Guarda el resultado actual por número de programa, muestra la lista y permite volver a abrir o borrar cada copia.
- Permite borrar un comentario individual y vaciar el historial con confirmación en dos pasos.
- Permite destacar un comentario activo haciendo clic o enfocando su tarjeta y pulsando Enter/Espacio; el historial muestra el envío y marca el comentario seleccionado.
- Exporta los resultados filtrados a CSV UTF-8 con BOM para Excel y protege celdas que podrían interpretarse como fórmulas; crea respaldos con los endpoints compartidos de Cortex.
- La consulta agregada del Laboratorio de Rulo resume eventos reales del historial activo por período/cuenta/día y los chats guardados por programa. Excluye eventos sintéticos del historial activo, no lee ni devuelve autores, textos, `externalId` o `participantId`; los archivos se informan aparte porque pueden repetir comentarios ya activos. El filtro de período/cuenta solo aplica al historial activo; el resumen de archivos cubre todos los períodos/cuentas, con totales generales y los 20 programas con más comentarios. La ruta no admite SQL, búsquedas por persona ni escritura.
- Conserva el estilo del historial anterior: cabecera plegable, tema oscuro/claro, controles de tamaño de texto, franja de estado, filas alternadas y diseño adaptable a móviles.

## Overlay para OBS

Se reconstruyó el flujo visual de `Rulo/rulo-chat-overlay.html`: fondo transparente, tarjetas azul noche con placas azules para los nombres, mensajes nuevos abajo, límite de 20 mensajes, desvanecido a los 30 segundos y orientación superior mediante `reverse`. El destacado conserva el aspecto art-forest del overlay antiguo: tarjeta clara verde, tipografía serif para el autor, formas asimétricas y el canguro vectorial original, copiado a `web/facebook/assets/kangaroo.svg` sin la declaración DTD externa. La imagen se sirve desde Cortex Go, sin CDN. La versión antigua también recibía objetos enriquecidos de SocialStream Ninja (avatares, insignias, donaciones, imágenes y otras plataformas); esos datos no existen en el capturador Facebook de Go. El overlay nuevo muestra únicamente campos disponibles: autor, comentario, cuenta, ID de transmisión y hora original opcional. No depende de SocialStream Ninja ni de su sesión.

El panel y el historial muestran un único acceso **Overlay OBS**, que abre `/rulo-chat-overlay.html` con chat y destacados integrados. La fuente de navegador `/rulo-featured.html` (también `/modules/facebook/featured`) sigue disponible si una escena quiere mostrar solo el último destacado. En el historial se puede tocar una tarjeta o pulsar **Destacar**, tanto en el chat actual como en una copia guardada. Cortex resuelve el comentario desde SQLite y envía el registro a las fuentes SSE; el overlay combinado lo presenta en una tarjeta central y quita esa misma tarjeta de la lista normal para evitar duplicados. La fuente independiente muestra la misma selección. La interfaz confirma el envío y muestra el estado de error si falla.

Los mensajes del chat limitan el área de lectura a `min(34vh, 260px)`; el destacado usa `min(45vh, 360px)`. Cuando el texto supera el área, queda arriba durante la pausa configurada y luego se desplaza hasta el final a la velocidad configurada. Las tarjetas normales respetan `showtime` para que los mensajes largos no se acumulen; el destacado permanece hasta completar el recorrido y mantiene una pausa final breve. `showtime=0` desactiva el vencimiento. Los comentarios del servidor admiten hasta 4096 caracteres. La preferencia del sistema para reducir movimiento detiene el balanceo decorativo del canguro.

El chat admite parámetros visuales opcionales: `limit=20` (1–60), `showtime=30000` (milisegundos; 0 desactiva el vencimiento), `scrollDelay=4500` (milisegundos; 0–20000), `scrollSpeed=18` (píxeles por segundo; 4–60), `reverse`, `fadezone=160` (píxeles), `width=350` (240–900), `chatHeight=260` (altura del texto normal en píxeles; 80–1000), `featuredHeight=360` (altura del texto destacado en píxeles; 100–1200), `scale=1` (0,65–1,8), `feedId=a|b`, `streamId=...` y `time=1`. Las alturas se limitan al espacio visible de la pantalla; si el texto las supera, se desplaza. La fuente independiente de destacados también acepta `scrollDelay`, `scrollSpeed` y `featuredHeight`; el destacado extiende su duración si hace falta para terminar el recorrido. `debug=1` muestra el estado SSE, `preview=1` presenta una tarjeta de ejemplo sin conectarse al capturador y `preview=1&sample=long` abre una muestra de texto más corta, solo en la tarjeta destacada. El botón **Más opciones** del panel central permite editar límites, tiempo en segundos, pausa, velocidad, ancho, altura del texto normal y destacado, cuenta, transmisión, orden, hora y diagnóstico; genera una URL copiable para OBS y guarda esos ajustes en el almacenamiento local de ese navegador. Una URL generada con el host de Tailscale conserva ese mismo host para abrirla desde el teléfono. Un acceso nuevo muestra solo comentarios posteriores a su conexión; si SSE se reconecta, recupera eventos guardados desde su cursor.

El SSE entrega filas ya persistidas en la misma tabla `events`; los eventos sintéticos del simulador también pueden aparecer. Una cola limitada evita crecimiento sin límite: si se llena, Cortex cierra esa conexión y el navegador vuelve a conectarse recuperando eventos desde SQLite. El texto se inserta como texto plano y conserva entidades y emojis sin ejecutar HTML recibido. No se guardan preferencias propias del overlay: los ajustes viven en la URL de la fuente. La selección destacada se mantiene en memoria durante la ejecución de Cortex; el comentario original permanece en SQLite y se puede volver a destacar después de un reinicio.

El historial activo usa la tabla `events` y los mismos endpoints SQLite de captura y borrado. La migración v3 agrega `event_archives` a esa misma base para guardar copias por programa; la v4 crea índices para filtrar por fecha y transmisión. Una copia no cambia al borrar comentarios activos. Cada archivo puede contener hasta 5000 comentarios y 8 MiB. El operador puede borrar las copias una a una. Los respaldos anteriores también permanecen independientes. El CSV contiene nombres y textos y se descarga localmente.

## Operación y límites

Las respuestas de Rulo aparecen debajo del comentario que las originó en el historial activo, enlazadas por `eventId`. El overlay combinado usa la misma conexión SSE y recupera el estado vigente al reconectar; una actualización tardía muestra una notificación breve con el resultado nuevo. Silenciar la presentación desde el laboratorio no borra ni oculta las respuestas en el historial. Este adaptador no publica comentarios en Facebook.

1. Inicia Cortex Go en la computadora donde se ejecutan los perfiles de Facebook.
2. Abre una de las rutas del historial desde esa computadora o mediante la URL privada de Tailscale del host.
3. Revisa que el estado indique **Captura activa**; para empezar a recibir mensajes, la ventana observada debe mostrar Comentarios del Live Producer.
4. Usa búsqueda, cuenta, fechas, ID de transmisión y páginas para encontrar comentarios anteriores. La retención activa continúa hasta que el operador borre los comentarios.
5. Si estás viendo comentarios antiguos, el aviso flotante indica cuántos nuevos llegaron. Pulsa el aviso cuando quieras regresar a los más recientes.
6. Para conservar una sesión aparte, elige **Guardar chat actual** e indica el número del programa. Los filtros activos limitan qué comentarios entran en esa copia.

La pantalla y el overlay no confirman que Facebook haya entregado todos los comentarios; reflejan solo los que la sesión cargó y el detector envió. Las acciones de respuesta/destacar del bot, avatares personales y los controles de Spotify/OBS dependen de servicios del Cortex anterior que todavía no existen en Go. La aceptación de login y comentarios reales requiere probarla en la computadora del capturador. La URL del overlay y su conexión SSE están implementadas, pero todavía falta revisar tamaño, escala y legibilidad con la escena y resolución reales de OBS.
