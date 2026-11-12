# Auditoría de OBS y relación con Cortex

**Fecha:** 12 de noviembre de 2026 (Australia/Brisbane)  
**Estado:** parcial; solo lectura de la ruta indicada.  
**Alcance:** archivos de perfil OBS indicados por el propietario.  
**Criterio:** los archivos presentes muestran configuración, no prueban qué está cargado o se usa en una transmisión.

## Hallazgos confirmados

- El ejecutable bin/64bit/obs64.exe identifica **OBS Studio 33.0.1**.
- La carpeta del perfil principal contiene exactamente tres archivos: basic.ini, service.json y streamEncoder.json. No contiene colecciones de escenas, recursos multimedia, registro de uso ni configuración visible de plugins en esa ruta.
- basic.ini registra modo de salida avanzada y lienzo/salida de **720×1280 a 30 FPS**. También registra opciones de reconexión. La demora aparece deshabilitada aunque hay un valor de segundos configurado. Es una instantánea del archivo, no confirmación de perfil activo.
- service.json contiene type y settings; no inspeccioné los valores de settings, que pueden incluir datos de transmisión. No se copiaron credenciales.
- En obs-plugins/64bit aparecen DLL de 3d-effect, freeze-filter, gradient-source, obs-advanced-masks, obs-composite-blur, obs-noise, obs-stroke-glow-shadow, waveform y sentry-wer. Esto confirma archivos instalados, no que OBS los cargue ni que se utilicen.
- La instalación incluye scripts distribuidos con OBS bajo core/frontend-tools/data/scripts (pausa de escena, instant replay, cuenta regresiva, reloj y texto de URL). No se encontraron scripts personalizados del usuario en la ruta examinada.

## Respuesta a la indicación del propietario

El propietario pidió revisar una carpeta local de perfil de OBS. Se revisó recursivamente: allí están basic.ini, service.json y streamEncoder.json. Esa ubicación contiene un perfil de salida, pero **no una colección de escenas**. No permite confirmar escenas o fuentes.

## Matriz OBS ↔ Cortex

| Área de OBS | Evidencia actual | Función de Cortex asociada | Estado/impacto |
|---|---|---|---|
| Perfil y salida de video | basic.ini; 720×1280, 30 FPS, modo avanzado. | Paneles y preparación de directo; Trigger Studio solo indirectamente. | Configuración observada; no confirmada como activa ni como requisito invariable. |
| Servicio de transmisión | Existe service.json; contenido sensible sin inspeccionar. | Ningún comportamiento específico puede deducirse de forma segura. | No verificado; no incluir secretos ni perfil de servicio en documentos compartibles. |
| Colecciones/escenas/fuentes | No están en la ruta indicada. | Overlays de Rulo, letras, comentarios, banner, clima/luces, lotería, Trigger Studio. | No se puede trazar fuente, escena, orden, visibilidad ni URL exacta. |
| OBS WebSocket | Documentación de Cortex/Trigger Studio menciona puerto 4455 y autenticación. | Trigger Studio, diagnóstico de Cortex y acciones de hotkey. | Configurado según documentos; conexión en vivo no probada. |
| Plugins | Se listan nueve DLL de 64 bits. | Los efectos podrían cambiar cómo se ven overlays o fuentes. | Instalados; relación con Cortex y uso desconocidos. |
| Scripts | La instalación trae ejemplos de OBS. | Puede haber automatización que no dependa del backend. | No se confirmó habilitación ni uso. |
| Medios externos | No inventariados. | Rulo 3D, clips, avatares, pistas, recursos Browser Source. | Dependencias pendientes; la exportación de escenas tampoco garantiza incluir archivos externos. |
| Operación real | No observada. | Todo control/overlay que se muestre o cambie durante el directo. | Frecuencia, pasos y criticidad pendientes del propietario. |

## Límites y próxima evidencia útil

No se abrió OBS ni se intentó conectar a una emisión. No se inspeccionó %APPDATA%\obs-studio ni se asumió que coincida con esta instalación. No se modificó OBS, sus fuentes, escenas, perfiles o plugins.

Para completar la trazabilidad haría falta una copia de solo lectura/exportación de las colecciones de escenas y del perfil realmente usado, con claves de transmisión, contraseña de WebSocket, URLs privadas y nombres personales revisados/eliminados. También hace falta confirmar qué paneles y escenas se usan habitualmente y en situaciones especiales. Hasta entonces, ninguna función queda marcada para retirar por “no aparecer” en esta carpeta.

