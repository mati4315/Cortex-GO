# Auditoría del Cortex vigente

**Fecha:** 12 de noviembre de 2026 (Australia/Brisbane)  
**Estado:** parcial; inspección estática de solo lectura.  
**Fuentes:** código y documentación del sistema anterior, examinados localmente. No se inició ningún servicio ni se accedió a datos productivos.

## Resumen ejecutivo

**Confirmado:** el Cortex actual no es un único programa ni un único proceso. Es un conjunto de servicios Node.js con dashboards web, una extensión de navegador, módulos TypeScript, utilidades Python y múltiples persistencias e integraciones. El README principal describe Control Cortex como backend central y otros servicios como componentes administrables.

**Confirmado por inventario estático:** fuera de dependencias y carpetas Git se encontraron 96 archivos .js, 6 .ts y 3 .py; no se encontraron archivos fuente Go, C#, Rust, PHP ni C++ en el repositorio. También hay HTML, CSS, scripts de Windows y artefactos compilados. El conteo incluye pruebas, utilidades y componentes de terceros; no representa el tamaño de la aplicación activa.

**Confirmado:** se encontraron 14 archivos package.json fuera de node_modules y .git. Incluyen servicios, pruebas, el servidor de Trigger Studio, su cliente React extraído y componentes de SocialStream Ninja. No todos son parte del Cortex que se debería reconstruir.

**Confirmación del propietario:** usa todos los módulos actuales salvo SocialStream Ninja, que no se usa y se debe excluir. Quiere simplificar y unificar los demás. Esto confirma su alcance funcional, pero no el detalle ni la frecuencia de cada flujo.

**Incidencia reportada por el propietario:** el servidor de Control Cortex a veces se cierra o requiere reinicio; hoy se recupera ejecutando `Control Cortex/Iniciar Control Cortex.bat`. No se determinó la causa ni la frecuencia con lectura estática.

**Probable, según código y documentación:** el mayor costo del rediseño sería consolidar responsabilidades y reconstruir límites de datos e integraciones; reemplazar Node por otro lenguaje, por sí solo, no demuestra menor consumo ni menos fallos.

## Componentes encontrados

| Componente | Función documentada | Tecnología/entrada | Estado de evidencia |
|---|---|---|---|
| Control Cortex | Backend central, APIs, paneles, configuración compartida, diagnóstico, perfiles, modo de pruebas, trazas técnicas y gestión de servicios. | Node.js, Express, WebSocket, HTML/CSS/JavaScript; OBS WebSocket. | Implementado según documentación y rutas fuente; operación actual no comprobada. |
| Chat Bot Spotify | Extensión propia que lee comentarios de Facebook renderizados en Brave/Chrome; conecta con Cortex y Spotify. | Node.js, JavaScript de extensión; OAuth PKCE; SSE y HTTP local. | Implementado/documentado; depende de cambios del DOM de Facebook. |
| Rulo | Overlays, chat, historial, dashboards, animaciones, entrenamiento y simulación aislada. | Backend servido por Cortex; HTML/CSS/JavaScript; opcionalmente ejecutable RULO externo. | Implementado/documentado; uso real pendiente. |
| Rulo - Cuaderno Vivo | Perfil de tono, memoria local por identidad estable, propuestas revisables y recuperación de recuerdos en simulación. | JavaScript y SQLite; conexión opcional con proveedor de IA. | Primera etapa documentada; varias capacidades futuras están explícitamente pendientes. |
| comentarios | Overlay/dashboard de participantes, ruleta, importación manual desde SQLite, asignación y corrección manual de premios. | Node.js, Express, Socket.IO/WebSocket; puente a Cortex y Firebase/Cdelu. | Implementado/documentado; no se verificaron datos ni entregas reales. |
| Trigger Studio | Catálogo de medios, miniaturas, reproducción y acciones sobre OBS. | El servidor mantiene HTTP/WebSocket y conexión a OBS; el panel puede abrirse local o remotamente. OBS puede estar en esa PC o en otra; el overlay carga medios desde el servidor. | Implementado/documentado. El propietario confirma que actualmente usa Cortex, captura Facebook, Spotify y OBS en una PC; quiere controlar una segunda PC en el futuro. Trigger Studio puede exponerse como sección remota del Cortex sin servidor separado por defecto. Su panel/WebSocket actual no tiene autenticación propia; no exponerlo tal cual. |
| Modulo de loteria | Overlay, WebSocket y sincronización/cola relacionados con lotería. | Node.js y WebSocket; el README principal documenta el puerto 688. | Componente separado; la configuración de red y su uso requieren validación. |
| Modulo de sorteo scraping | API/dashboard de resultados IAFAS, ventanas horarias, MySQL y opción de sincronización externa. | Node.js, Express, MySQL, JWT/bcrypt y scheduler. | El propietario incluye este módulo en el alcance; decidir si el nuevo Cortex integra sus capacidades o conecta con el servicio remoto. |
| Bienvendos y Banner | Overlay y panel de bienvenida/banner. | Node.js, Express, Socket.IO. | Registrado como servicio; uso real pendiente. |
| luces | Controlador y overlay de luces interactivas. | Node.js, Express, Socket.IO. | Registrado como servicio; uso real pendiente. |
| modulo de tiempo y clima | Overlay de reloj, tiempo y clima. | Node.js, Express, Socket.IO. | Registrado como servicio; uso real pendiente. |
| SocialStream Ninja | Integración de terceros y compatibilidad heredada. | Proyecto de terceros con submódulos/overrides. | El propietario confirma que no se usa y pide excluirlo del diseño nuevo, incluidos sus servicios y dependencias heredadas. Antes de desarrollo, revisar referencias residuales en los módulos conservados. |
| Multi-Streaming upcloud | Relay de una salida OBS hacia dos cuentas de Facebook, reenviado por un VPS MediaMTX sin recodificar. Hay scripts para guardar claves, desplegar y consultar el servicio y mostrar configuración de OBS. | OBS envía SRT cifrado; el relay reenvía a dos destinos RTMPS. | Incluido en el alcance por la confirmación del propietario. Flujo descrito por README, no observado en vivo. Faltan frecuencia, PC responsable, recuperación y rotación de claves. No se leyeron archivos de claves ni valores de conexión. |

## Servicios administrables y puertos documentados

Control Cortex/backend/services.json registra ocho servicios secundarios: Luces (3758), Clima y Tiempo (3757), Bienvenidas y Banner (9344), Módulo de lotería, Módulo de sorteo scraping (8872), Ruleta de Comentarios (9742), TriggerStudio (2188) y Chat Bot Spotify (9765). El registro muestra stopped como estado configurado, pero eso no comprueba el estado de procesos en Windows.

La documentación identifica Control Cortex en el puerto 4000 y OBS WebSocket en el 4455. **No se verificó** que esos puertos estén escuchando, que los servicios estén instalados, ni que WebSocket esté autenticado durante esta auditoría.

## Persistencia y datos

- **SQLite de audiencia:** Rulo/Spotify/spotify-analytics.db. El código documenta consultas históricas, pedidos Spotify, aprendizaje revisable, simulaciones, instantáneas de lotería y auditoría de premios. No se abrió la base ni se leyeron filas.
- **Historial visual:** Rulo/rulo-chat-history.json, con límite documentado de 200 elementos; no reemplaza SQLite.
- **Cuaderno Vivo:** SQLite local y clave HMAC de identidades que deben conservarse/restaurarse juntas. La documentación señala propuestas revisables y borrado/corrección manual.
- **Perfiles del panel:** Control Cortex/backend/cortex-home.json; respaldos de recuperación y configuraciones locales. No se leyó su contenido.
- **Otros datos:** archivos JSON por módulo, SQLite de funciones específicas, MySQL remoto, Firebase/Cdelu, medios y avatares. Formatos y relevancia exacta requieren validar con el propietario.
- **Copias:** documentación de Cortex indica snapshots SQLite consistentes al inicio, cada 24 horas y a demanda, con siete copias. No se verificó su existencia ni una restauración.

## Integraciones identificadas

Facebook mediante DOM de navegador; Spotify Web API y cliente de escritorio; OBS WebSocket; Firebase/Cdelu para premios; servicio IAFAS/MySQL; proveedores de IA configurables; Tailscale y, opcionalmente, ngrok de solo lectura; navegador Brave/Chrome; ejecutable externo RULO; FFmpeg en Trigger Studio. El propietario confirma que actualmente Cortex, captura Facebook, Spotify y OBS corren en una PC; el control futuro desde otra PC es un requisito por definir.

Son dependencias **documentadas**, no conexiones probadas en esta sesión.

## Cambios actuales del repositorio

El estado Git del sistema anterior informó una modificación previa en una configuración local de Spotify. No se leyó su contenido ni se modificó. El nuevo proyecto debe tratar la versión actual como referencia inmutable durante el análisis y la construcción.

## Alcance que sigue pendiente

No se midieron CPU, RAM, latencia o estabilidad. No se consultaron procesos, tareas programadas, firewall, drivers, hardware, bases de datos, secretos, medios externos ni sesiones reales. No se ejecutaron pruebas. No se comprobó qué módulos usa el propietario antes, durante, después o en eventos ocasionales. Estas ausencias impiden afirmar que la auditoría funcional está completa.

## Fuentes locales revisadas

- README.md, AGENTS.md, CONTEXTO DE TODO PRINCIPAL.md, CAMBIOS-PROYECTO.md.
- Control Cortex/CORTEX-INICIO.md, Control Cortex/backend/db.js, Control Cortex/backend/spotify-analytics.js, Control Cortex/backend/RESPALDOS-RULO.md.
- Chat Bot Spotify/README.md y manifiesto package.json.
- Rulo/README.md, Rulo/README-SIMULACION-VIVOS.md.
- Rulo - Cuaderno Vivo/README.md y ARQUITECTURA-Y-MIGRACION.md.
- Trigger Studio/README.md y Trigger Studio/docs/ARQUITECTURA_Y_DESARROLLO.md.
- comentarios/server.js, Modulo de sorteo scraping/README.md, manifiestos package.json y registro Control Cortex/backend/services.json.
- Multi-Streaming upcloud/README.md; se leyó la descripción de flujo sin abrir los archivos de claves ni operar el VPS.



