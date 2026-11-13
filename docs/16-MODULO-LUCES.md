# Módulo Luces

**Estado:** integrado en Cortex Go con servidor Go y páginas incluidas en el ejecutable.
**Referencia:** módulo Luces del Cortex anterior. Se adaptaron las pantallas y el renderizado visual; no se copiaron archivos de configuración, paquetes Node ni configuraciones guardadas.

## Acceso

- Panel de control: `http://127.0.0.1:17845/modules/luces/admin`
- Overlay para OBS: `http://127.0.0.1:17845/modules/luces/overlay`

Las mismas rutas funcionan bajo el host HTTPS de Tailscale Serve para dispositivos autorizados. La fuente de navegador debe usar la resolución base de la escena de OBS; el diseño se ajusta al tamaño disponible y mantiene el fondo transparente.

## Controles recuperados

- Presets Automático, Electronic, Disco Retro, Chill Out, Psicodélico y Solo Lasers.
- Modo aleatorio que cambia los parámetros cada pocos segundos mientras el panel permanece abierto.
- El modo aleatorio se pausa al ocultar el panel y retoma al volver a la pestaña, evitando cambios y escrituras en segundo plano.
- Velocidad general, intensidad, niebla, seis paletas y dos colores personalizados.
- Bola de espejos con velocidad y puntos proyectados.
- Cabezales móviles con cantidad, barrido, círculos, sincronización o baile, gobo y velocidad.
- Láseres con cantidad, barrido, onda, X o espiral, color y velocidad.
- Estrobo por frecuencia de 2–3 Hz o por beat, con color configurable; empieza desactivado y el panel muestra una advertencia sobre destellos rápidos.
- Wash LED, BPM virtual, tap tempo, pulso manual y sincronización opcional por micrófono.
- Vista previa en el panel, estado de overlays conectados y copia de la URL de OBS.
- Botón **Blackout** en el panel fijo: apaga temporalmente todos los overlays conectados y reactiva los efectos sin cambiar la configuración guardada.
- Vista previa en vivo durante el movimiento de sliders sin escritura en SQLite; al soltarlos se persiste una sola vez, en orden, con indicador de guardado o error.
- Enlace **Diagnóstico OBS** o `overlay?debug=1`, con FPS, resolución Canvas, conexión SSE, estado Blackout y preferencia de movimiento.

## Ritmo y audio

El BPM virtual está activo por defecto y funciona sin pedir permisos. El modo audio-reactivo es opcional: al activarlo, la fuente del overlay muestra un botón para que el navegador solicite acceso al micrófono. El audio se analiza localmente en esa página para detectar graves; Cortex solo recibe eventos de beat, nunca el audio.

La captura de micrófono depende de que el navegador permita `getUserMedia` y de que el usuario conceda el permiso. Si se deniega, el módulo vuelve al beat virtual.

## Persistencia y arquitectura

La configuración se guarda en la tabla `module_settings` de SQLite bajo el identificador `lights`, por lo que entra en los respaldos existentes. Panel y overlay comparten la configuración en tiempo real mediante eventos SSE del servidor Go. El beat manual se envía como una acción del mismo origen y se transmite a las demás páginas conectadas.

Blackout es un estado temporal del servidor y se sincroniza por SSE; no cambia ni guarda el preset, y vuelve a apagado cuando se reinicia Cortex. Durante el movimiento de controles, la configuración de vista previa se sincroniza sin tocar SQLite; al soltar un slider o pasar el panel a segundo plano, se persiste el estado final una vez. Si el panel que inició la vista previa se desconecta antes de guardar, el overlay vuelve a la configuración persistida. Las escrituras se serializan y las vistas previas intermedias se agrupan para evitar solicitudes duplicadas o fuera de orden.

El overlay conserva el renderizado Canvas del diseño anterior para dibujar las luces en el navegador de OBS. El backend, la persistencia y la coordinación de estado son Go; no se inicia un servidor Node, no se necesita Socket.IO y no se cargan fuentes desde CDN.

El renderizado Canvas y el análisis del AudioContext se pausan cuando el navegador oculta la página, reduciendo trabajo en pestañas no visibles. El módulo también ralentiza el movimiento y pausa el estrobo cuando el sistema solicita reducir movimiento. La frecuencia configurada para el estrobo se calcula por tiempo y no depende del ritmo de cuadros del navegador.

El rango del estrobo queda limitado a un máximo de 3 destellos por segundo por seguridad visual, tanto en modo de frecuencia como en modo por beat. Si la base ya tenía un valor superior, Cortex lo baja a 3 Hz al cargar la configuración.

El modo de diagnóstico se activa solo al abrir `/modules/luces/overlay?debug=1`; la fuente normal de OBS no muestra la superposición técnica. FPS sirve para comparar la fluidez de la fuente, mientras que el uso real de CPU se revisa en OBS o en el sistema operativo.

## API interna

- `GET /api/modules/lights/state`
- `GET` y `PUT /api/modules/lights/settings`
- `POST /api/modules/lights/preview` — sincronizar cambios temporales mientras se arrastran controles
- `GET /api/modules/lights/events` — flujo SSE para panel y overlays
- `POST /api/modules/lights/beat` — pulso manual o audio-reactivo
- `POST /api/modules/lights/blackout` — activar o restablecer el apagado temporal

El servidor valida enumeraciones, colores, rangos numéricos y cantidad de luces antes de persistir cambios.

## Alcance y aceptación

Luces genera un overlay visual para OBS. No controla luminarias físicas por DMX ni envía órdenes a OBS. La aceptación visual final debe hacerse con la fuente de navegador cargada en OBS, en una escena de prueba y en la resolución objetivo. La máquina actualmente disponible informa OBS desconectado, así que la apariencia dentro de OBS aún debe verificarse cuando esté abierto.
