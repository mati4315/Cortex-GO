# Módulo Clima y Tiempo

**Estado:** integrado en Cortex Go. La validación visual final dentro de una escena OBS queda pendiente hasta tener OBS disponible.

## Accesos

- Panel: `/modules/clima/admin`
- Fuente de navegador OBS: `/modules/clima/overlay`
- Vista previa local con fondo: `/modules/clima/overlay?preview=1&bg=checkerboard`
- Estado: `GET /api/modules/weather/state`
- Ajustes: `GET|PUT /api/modules/weather/settings`
- Acciones de rotación: `POST /api/modules/weather/control`
- Actualizar clima ahora: `POST /api/modules/weather/refresh`
- Eventos en vivo: `GET /api/modules/weather/events`

Las páginas y API se sirven desde Cortex Go; no inicia un servidor Node separado. El panel guarda los ajustes en `module_settings`; las lecturas meteorológicas y el último índice seleccionado se guardan en `module_runtime` dentro de la misma SQLite y entran en los respaldos generales.

## Funciones reconstruidas

- Lista ordenable de ubicaciones, etiqueta, ciudad, zona horaria, bandera y visibilidad.
- La rotación de vistas arranca activada por defecto; se puede pausar y reanudar desde el panel.
- Interruptores independientes para reloj y clima por ciudad. Cada reloj y lectura del clima entra en la rotación como una vista distinta.
- Formato de 12 o 24 horas, duración de 3–60 segundos, pausa, reanudación, vista anterior/siguiente y selección directa.
- Diseño tarjeta o cinta, cuatro posiciones, color, cinco presets visuales, seis transiciones, seis familias tipográficas y etiqueta «EN VIVO» configurable.
- Vista previa interactiva con fondos de cuadrícula, gris OBS o gameplay.
- Temperatura actual, icono/condición y distinción día/noche usando códigos WMO. Geocoding acepta `Ciudad, Región` para elegir entre localidades con el mismo nombre.

Se conservaron como punto de partida las tres ubicaciones y preferencias visibles en la configuración del módulo antiguo: Gold Coast, Concepción del Uruguay y San Justo; tono ámbar, tarjeta clásica y rotación pausada. No se copió su servidor Node ni su Socket.IO.

## Integración y mejoras técnicas

- Go mantiene el estado de rotación en un único planificador del servidor. Varias fuentes OBS y el panel muestran el mismo índice, sin ejecutar temporizadores independientes que aceleren o desincronicen la rotación.
- Server-Sent Events distribuye cambios de configuración, lecturas y estado de conexión; una reconexión recibe el estado completo actual.
- Las consultas se agrupan por ciudad, se ejecutan con concurrencia limitada y usan caché de diez minutos. La actualización automática ocurre cada diez minutos; el panel ofrece una acción manual.
- Una falla de red conserva la última lectura conocida como obsoleta, junto con el error, en vez de reemplazarla por una temperatura vacía. Las lecturas se guardan en SQLite y sobreviven al reinicio.
- El proveedor recibe solo el texto de ciudad para geocoding y coordenadas para el pronóstico. No se requieren claves. Las zonas horarias IANA se incluyen desde `time/tzdata`, lo que evita depender de una base de zonas horarias instalada en Windows.
- El panel valida campos, colores, zonas horarias, IDs únicos y límite de ubicaciones; los recursos web se sirven desde el ejecutable y no dependen de Google Fonts, CDN ni cargas de imágenes de banderas de terceros.
- El overlay muestra junto al reloj la bandera de Argentina o Australia como imagen SVG local; se oculta cuando se elige “Sin bandera”. No depende del CDN usado por el overlay legado.

Datos actuales consultados en [Forecast API de Open-Meteo](https://open-meteo.com/en/docs) y [Geocoding API de Open-Meteo](https://open-meteo.com/en/docs/geocoding-api). Los nombres de lugares se basan en GeoNames.

## Mejoras candidatas

- **Selector de ciudad con resultados explícitos.** El campo conserva el formato anterior `Ciudad, Región`; si el proveedor no devuelve una coincidencia exacta para la región, hoy se usa el primer resultado disponible. Una próxima mejora segura sería mostrar opciones con ciudad, región y país, y guardar las coordenadas elegidas para evitar ambigüedad y geocodificaciones repetidas. Mantén la entrada de texto existente como alternativa compatible.
- **Unidades configurables.** El módulo anterior mostraba Celsius. Fahrenheit puede agregarse como opción si el propietario la pide, sin cambiar el valor predeterminado ni la presentación actual.

## Revisión pendiente

El ejecutable compila con este módulo integrado. Antes de dar por aceptado el overlay en OBS, revisar escala, márgenes, transparencia, fuente y legibilidad en la resolución real de la escena. El panel remoto y el enlace del overlay dependen de que esta versión de Cortex Go esté desplegada y abierta en el host activo.
