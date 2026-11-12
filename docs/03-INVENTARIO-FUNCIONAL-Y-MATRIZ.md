# Inventario funcional y matriz Cortex ↔ OBS ↔ uso real

**Fecha:** 12 de noviembre de 2026 (Australia/Brisbane)  
**Estado:** el propietario confirma que usa todos los módulos salvo SocialStream Ninja; aclaró el flujo general del directo y necesita reconocer comentarios de dos cuentas de Facebook. Frecuencia y criticidad por flujo aún pendientes.  
**Fuentes:** repositorio actual y ruta OBS indicada; sin observación de transmisión.

## Confirmación de alcance del propietario

El propietario confirma que usa todos los módulos del Cortex actual excepto SocialStream Ninja, que no usa y quiere eliminar. Por tanto, todas las demás capacidades quedan dentro del alcance de Cortex 2.0; la meta es simplificarlas y unificarlas, no descartarlas. La frecuencia, el orden de uso y la PC responsable de cada flujo todavía requieren detalle.

## SocialStream Ninja queda excluido

No se reconstruirán ni incorporarán su servidor, extensión, relay, overrides o dependencia. Antes de implementar, revisar las referencias heredadas en los módulos conservados y asignar un dueño nuevo a cualquier comportamiento que el propietario sí use.

## Funciones que aparecen en la versión actual

| Función/flujo | Evidencia en el sistema actual | Relación con OBS | Uso real |
|---|---|---|---|
| Panel principal y gestión de módulos | Control Cortex sirve paneles, estado, perfiles operativos, inicio/reinicio y comprobaciones previas al directo. | Comprueba WebSocket/escena/fuentes cuando se configura; puede abrir paneles remotos. | Módulo en uso por declaración del propietario; frecuencia y casos exactos pendientes. |
| Estabilidad del servidor central | El propietario informa que a veces el servidor Control Cortex se cierra o requiere reinicio; hoy vuelve a iniciar el lanzador del sistema anterior. | Al cerrarse el servidor se pierde el punto de control hasta que lo vuelven a lanzar. | Falla observada por el propietario; causa y frecuencia sin diagnosticar. El nuevo sistema debe detectar la terminación, recuperarse mediante un supervisor independiente y conservar un registro del motivo. |
| Captura de comentarios de Facebook | Extensión propia lee el DOM visible de Facebook en Brave/Chrome y manda eventos a Chat Bot Spotify y Cortex. El propietario requiere reconocer comentarios de los vivos de sus dos cuentas. No depende del capturador de SSN. | Comentarios alimentan historial, Rulo y destacados. | Módulo en uso y objetivo de dos cuentas confirmado; probar estado individual, cuenta correcta, último evento y recuperación. La captura actual depende del navegador. |
| Pedidos de Spotify | OAuth PKCE propio; cola serial, comprobación de pedido duplicado, dispositivo y URI reproduciéndose; cooldowns y offset documentados. | Puede enviar respuesta al overlay/voz; el reproductor puede estar en la PC de streaming. | Módulo en uso por declaración; confirmar frecuencia por directo. Mantener verificación de reproducción. |
| Modo de pruebas aislado | Los documentos describen pausa de pedidos automáticos y separación de SQLite, historial, avatares y ruleta. | Evita pruebas que alteren el chat/overlay real o reproduzcan audio sin intención. | Módulo en uso por declaración; frecuencia y escenarios de prueba pendientes. |
| Rulo | Dashboard/overlay de respuestas, historial, mascota animada WebM o ejecutable externo, biblioteca de medios y entrenamiento. | Browser Sources y posible ejecutable/modelo en red local. | Módulo en uso por declaración; confirmar qué apariencia, animaciones y paneles forman parte del flujo. |
| Rulator / simulación de vivos | Simula programas históricos y comentarios sintéticos; las consultas privadas autorizadas son solo lectura; no publica en OBS ni guarda conversaciones en datos reales. | No debería publicar en OBS mientras esté aislado. | Módulo en uso por declaración; confirmar frecuencia y límites de consultas. |
| Cuaderno Vivo | Recuerdos por identidad estable en SQLite; propuestas IA revisables y editables; memoria recuperable en simulación si se activa. | El chat público actual conserva su motor mientras se valida. | Módulo en alcance por declaración; separar capacidades existentes de ideas futuras aún no implementadas. |
| Historial de audiencia | SQLite permanente, JSON visual reciente, avatares y CSV; respaldos automáticos/manuales descritos. | Datos alimentan dashboards, visualizaciones y participantes importados. | Módulo en uso por declaración; la base no se abrió y no se verificaron filas. |
| Comentarios y Ruleta | Importación manual de participantes desde SQLite; giro; premio solo después de confirmación del operador; corrección manual e idempotencia. | Overlay/dashboard OBS; premios consultan sistemas externos. | Módulo en uso por declaración; confirmar frecuencia, reglas y conexiones activas a Cdelu/Firebase. |
| Trigger Studio | Catálogo, miniaturas, acciones y reproducción controlada por WebSocket con OBS. El panel actual ya puede abrirse remotamente. | Cortex conecta a OBS por WebSocket; Browser Source puede cargar medios desde el host. | Módulo en uso. Todo corre hoy en la PC principal; la notebook futura será solo panel de control. Trigger Studio queda como sección central, sin backend/agent en la notebook. El panel vigente no tiene autenticación propia y no se debe exponer tal cual. |
| Banner, luces, clima y hora | Servicios con dashboards y overlays registrados en Cortex. | Habitualmente se muestran como Browser Sources, según documentación. | Módulos en uso por declaración; confirmar frecuencia individual. Estado del registro no prueba ejecución. |
| Lotería y datos IAFAS | Un módulo WebSocket/overlay y otro servicio de resultados IAFAS/MySQL aparecen en el árbol. | Overlay asociado a sorteos; conexión exacta de cada escena no vista. | Módulos en uso por declaración; confirmar integración, frecuencia y PC responsable. |
| Multi-Streaming UpCloud | El README describe OBS → SRT cifrado → relay MediaMTX en VPS → dos destinos Facebook por RTMPS, sin recodificar; hay scripts para claves, despliegue, estado y configuración de OBS. | Recibe la señal de salida de OBS y la replica a ambas cuentas. | Capacidad en alcance y flujo documentado; directo/estado en vivo no observado. Confirmar frecuencia, PC, recuperación y rotación. La URL SRT permite publicar; tratarla como secreto y no mostrarla en el panel/logs. No se leyeron valores de claves. |
| Túnel remoto | Documentación menciona panel privado por LAN/Tailscale y túnel ngrok limitado a lecturas. | No requiere que OBS cambie de escena; puede presentar estados/historial. | Requisito deseado documentado, no confirmación del uso actual. |

Los directorios de temas y recursos visuales que no aparecen como servicios ejecutables requieren una clasificación aparte: identificar qué se usa, qué función nueva lo servirá y si el propietario quiere conservar esos archivos. No se copian ni se descartan por inferencia durante esta fase.

## Matriz de trazabilidad

| Acción funcional | Origen/disparador documentado | Acción/salida | Recurso que falta observar | Estado |
|---|---|---|---|---|
| Destacar comentario | Operador desde historial/dashboard; comentario capturado desde Facebook. | Endpoint /api/featured y overlay dedicado. | Escena/fuente exacta, ubicación, duración y momento de uso. | Implementado/documentado; OBS y frecuencia no verificados. |
| Mostrar respuesta de Rulo | Respuesta del bot en chat. | Overlay de Rulo servido por Cortex. | Browser Source y filtros/visibilidad de OBS. | Documentado; no observado. |
| Letras/overlay musical | Reproducción Spotify y datos de pista. | Página overlay de letras/unificada. | Escena y fuente configuradas; proveedor de letras. | Documentado; no observado. |
| Cambiar escena por hotkey OBS | Ruta `POST /api/obs/trigger-hotkey`; usa `TriggerHotkeyByKeySequence` con las teclas configuradas (el código describe teclas numéricas 1/2/3). | OBS ejecuta la hotkey y cambia de escena según la configuración local. | Hotkeys y escenas asignadas, PC donde corre OBS, autenticación/estado WebSocket y confirmación visual. | Implementado en código; conexión, mapeo y uso real no verificados. |
| Ejecutar acción de OBS | Panel/Trigger Studio. | OBS WebSocket; endpoint de hotkey y control de medios. | Comandos configurados, autenticación y resultado visual. | Implementado/documentado; conexión no probada. |
| Rulo animado | Estado/evento de Rulo o elección manual de clip. | WebM en OBS o ejecutable externo en LAN. | Modelo, clips, ruta real, sincronización de audio y escena. | Documentado; recursos externos no inventariados. |
| Ruleta | Importación manual, giro y confirmación del operador. | Overlay de participantes; premios por ruta de Cortex/Cdelu. | Escena/fuente y conectividad con servicio externo. | Flujo documentado; operación no observada. |
| Banner/luces/clima/hora | Configuración manual o condiciones del módulo. | Dashboards/overlays locales. | URLs de navegador y activación por perfil. | Implementados según archivos; uso no confirmado. |
| Preparar/terminar directo | Selección de perfil operativo en Control Cortex. | Arranque de módulos, preflight y apagado seleccionado. | Pasos reales y cualquier proceso/dispositivo manual adicional. | Documentado; sesión real no observada. |
| Validar comentarios al iniciar | El operador desea abrir pocas cosas y comprobar los vivos de ambas cuentas antes de salir al aire. | Cortex muestra estado de captura y hora del último comentario por cuenta. | Una acción de inicio, PCs, pestañas y criterio de “reconocido” pendientes; la captura actual usa extensión de navegador. | Necesidad confirmada; mecanismo objetivo no probado. |
| Guardar y cerrar | El operador cierra todo al terminar y espera que la base haya guardado lo necesario. | Guardado durable y confirmación de cierre limpio. | Acordar eventos a guardar, retención, manejo de escrituras pendientes y respaldo. | Necesidad confirmada; criterio de aceptación pendiente. |
| Recuperar una falla | El operador quiere recuperación automática o un botón/comando de reinicio. | Reconectar o reiniciar componente; indicar el resultado. | Definir niveles, permisos y protección contra repetir acciones de OBS, Spotify, sorteos y bolilleros. | Necesidad confirmada; recuperación actual/nueva por especificar. |

## Qué conservar en el diseño nuevo

El único retiro confirmado es SocialStream Ninja. El propietario confirma que usa los demás módulos; sus capacidades siguen en el alcance aunque falte detallar frecuencia y conexión con escenas OBS. No se retirará ninguna de ellas por no aparecer en la carpeta de OBS.

La indicación del propietario de “crear desde cero y no migrar” significa que el código nuevo debe diseñarse de los requisitos y reescribirse; esta auditoría no copiará carpetas ni traducirá código. Tampoco se presupone una importación de SQLite, perfiles o historial. La conservación, importación selectiva o inicio sin historia previa son decisiones de datos separadas que requieren confirmación antes de desarrollo.

## Detalles operativos pendientes

El propietario confirmó que usa todos los módulos salvo SocialStream Ninja y quiere simplificar/unificar los restantes. Describió el flujo general: inicio mínimo y verificación de dos cuentas; operación manual de Rulo, bolilleros y comentarios destacados; cierre con datos guardados; recuperación automática o manual. Quedan pendientes frecuencia, PCs, pasos precisos y criterios de aceptación. Ninguna capacidad restante se marca como obsoleta. Las capturas se describen en [flujo operativo real](10-FLUJO-OPERATIVO-REAL.md).





