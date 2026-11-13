# Módulo Luces

**Estado:** integrado en Cortex Go con servidor Go y páginas incluidas en el ejecutable.
**Referencia:** `D:\plugins para mi OBS\luces`. Se adaptaron las pantallas y el renderizado visual; no se copiaron `config.json`, paquetes Node, `node_modules` ni configuraciones guardadas.

## Acceso

- Panel de control: `http://127.0.0.1:17845/modules/luces/admin`
- Overlay para OBS: `http://127.0.0.1:17845/modules/luces/overlay`

Las mismas rutas funcionan bajo el host HTTPS de Tailscale Serve para dispositivos autorizados. La fuente de navegador debe usar la resolución base de la escena de OBS; el diseño se ajusta al tamaño disponible y mantiene el fondo transparente.

## Controles recuperados

- Presets Automático, Electronic, Disco Retro, Chill Out, Psicodélico y Solo Lasers.
- Modo aleatorio que cambia los parámetros cada pocos segundos mientras el panel permanece abierto.
- Velocidad general, intensidad, niebla, seis paletas y dos colores personalizados.
- Bola de espejos con velocidad y puntos proyectados.
- Cabezales móviles con cantidad, barrido, círculos, sincronización o baile, gobo y velocidad.
- Láseres con cantidad, barrido, onda, X o espiral, color y velocidad.
- Estrobo por frecuencia de 2–3 Hz o por beat, con color configurable; empieza desactivado y el panel muestra una advertencia sobre destellos rápidos.
- Wash LED, BPM virtual, tap tempo, pulso manual y sincronización opcional por micrófono.
- Vista previa en el panel, estado de overlays conectados y copia de la URL de OBS.

## Ritmo y audio

El BPM virtual está activo por defecto y funciona sin pedir permisos. El modo audio-reactivo es opcional: al activarlo, la fuente del overlay muestra un botón para que el navegador solicite acceso al micrófono. El audio se analiza localmente en esa página para detectar graves; Cortex solo recibe eventos de beat, nunca el audio.

La captura de micrófono depende de que el navegador permita `getUserMedia` y de que el usuario conceda el permiso. Si se deniega, el módulo vuelve al beat virtual.

## Persistencia y arquitectura

La configuración se guarda en la tabla `module_settings` de SQLite bajo el identificador `lights`, por lo que entra en los respaldos existentes. Panel y overlay comparten la configuración en tiempo real mediante eventos SSE del servidor Go. El beat manual se envía como una acción del mismo origen y se transmite a las demás páginas conectadas.

El overlay conserva el renderizado Canvas del diseño anterior para dibujar las luces en el navegador de OBS. El backend, la persistencia y la coordinación de estado son Go; no se inicia un servidor Node, no se necesita Socket.IO y no se cargan fuentes desde CDN.

El renderizado Canvas y el análisis del AudioContext se pausan cuando el navegador oculta la página, reduciendo trabajo en pestañas no visibles. La frecuencia configurada para el estrobo se calcula por tiempo y no depende del ritmo de cuadros del navegador.

El rango del estrobo queda limitado a 2–3 Hz por seguridad visual. Si la base ya tenía un valor superior, Cortex lo baja a 3 Hz al cargar la configuración.

## API interna

- `GET /api/modules/lights/state`
- `GET` y `PUT /api/modules/lights/settings`
- `GET /api/modules/lights/events` — flujo SSE para panel y overlays
- `POST /api/modules/lights/beat` — pulso manual o audio-reactivo

El servidor valida enumeraciones, colores, rangos numéricos y cantidad de luces antes de persistir cambios.

## Alcance y aceptación

Luces genera un overlay visual para OBS. No controla luminarias físicas por DMX ni envía órdenes a OBS. La aceptación visual final debe hacerse con la fuente de navegador cargada en OBS, en una escena de prueba y en la resolución objetivo. La máquina actualmente disponible informa OBS desconectado, así que la apariencia dentro de OBS aún debe verificarse cuando esté abierto.
