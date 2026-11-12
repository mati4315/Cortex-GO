# Estado de auditoría, riesgos y decisiones pendientes

**Fecha:** 12 de noviembre de 2026 (Australia/Brisbane)  
**Estado general vigente:** Go fue elegido y el propietario autorizó el primer corte local, ya implementado y probado. La reconstrucción completa y las integraciones reales todavía no comenzaron. La segunda pasada de IA 2 fue en el mismo hilo y no equivale a revisión independiente.

**Decisiones vigentes:** proyecto nuevo en Go, sin portar código; SocialStream Ninja excluido; los otros módulos siguen en alcance. La PC principal ejecutará Cortex, OBS, Spotify y captura; la notebook será control remoto en una etapa posterior. El primer corte usa feeds ficticios y base nueva; no toca el sistema anterior ni usa comentarios reales.

## Recomendación de continuación (propuesta, todavía no es una decisión)

1. El propietario descartó WebView2. Mantener el panel web en el navegador predeterminado; no agregar ese runtime ni una ventana integrada salvo que el propietario cambie su preferencia.
2. Antes de guardar comentarios reales, versionar el esquema SQLite y diseñar/probar respaldo y restauración con datos sintéticos.
3. Integrar después la captura de las dos cuentas de Facebook como primer flujo real. Revisar cómo funciona hoy la captura y escribir un adaptador nuevo; si Facebook requiere acceso desde el DOM del navegador, puede hacer falta un puente JavaScript pequeño mientras Go conserva la coordinación y persistencia. Mostrar salud de captura y hora del último comentario por cuenta.
4. Probar un solo control reversible de OBS (por ejemplo, destacar un comentario en una fuente de ensayo). Incorporar Spotify y demás flujos luego; premios y acciones irreversibles al final, siempre con confirmación.
5. Habilitar la notebook solo después de definir autenticación y acceso por red privada. El primer corte está ligado a loopback y no se debe exponer el WebSocket de OBS directamente.

Conservar el programa viejo como respaldo hasta completar varias pruebas operativas del nuevo. Durante las pruebas reales, evitar que ambas versiones controlen a la vez las mismas cuentas, escenas o acciones para no duplicar eventos. Para secretos de producción, usar el almacén de credenciales de Windows o DPAPI, no archivos de configuración ni logs.

La matriz siguiente conserva el alcance de la auditoría documental original: “parcial” describe cuánto se inspeccionó del sistema viejo, no invalida las decisiones y pruebas posteriores registradas en [el primer corte](11-PRIMER-CORTE-PRUEBA-TECNICA.md).

## Matriz de cobertura

| Área requerida | Estado | Evidencia consultada | Qué falta / impacto |
|---|---|---|---|
| Guía de roles y criterios | Revisada en segunda pasada | Plan Cortex IA 1, versión 4.4, completo; dictamen condicional en 09. | La misma IA hizo ambas pasadas; pedir revisión independiente si se requiere esa garantía. |
| Código/documentación del Cortex | Parcial | Árbol, manifiestos, README, AGENTS, arquitectura y rutas seleccionadas. | Falta revisar exhaustivamente todos los módulos/casos de borde; no afirmar cobertura completa de cada archivo. |
| OBS | Parcial | OBS 33.0.1, un perfil y archivos de plugin de la ruta dada. | No hay colección de escenas ni prueba en vivo; no se confirma qué fuentes usa Cortex. |
| Windows y procesos | No auditado | No se inspeccionó el estado actual de procesos/servicios, tareas, firewall, drivers o dispositivos. | Necesarios para determinar dependencias, arranque y división entre PCs. |
| Integraciones | Parcial | Documentación y código de conexiones Facebook, Spotify, OBS, Cdelu/Firebase, IAFAS/MySQL, IA, Tailscale/ngrok. | No se probaron conexiones, credenciales, contratos actuales ni recuperación de fallos. |
| Datos y persistencia | Parcial | Esquemas y manuales localizados, sin abrir bases/filas ni archivos privados. | No se sabe qué datos quiere conservar el propietario; formato de importación no decidido. |
| Multimedia y recursos | Parcial | Referencias a clips, imágenes, perfiles de navegador y ejecutable externo en docs. | Assets fuera del repositorio, rutas, licencias y dependencias no inventariadas. |
| Alcance de módulos | Confirmado por el propietario | Usa todos los módulos salvo SocialStream Ninja; pide excluir SSN y simplificar/unificar los demás. | Faltan frecuencia, secuencia y PC responsable por flujo. |
| Flujo de directo | Parcialmente confirmado por el propietario | Inicio unificado, captura de dos cuentas, controles manuales, cierre y recuperación. | Pasos precisos, datos a persistir y significado de “reiniciar todo”. |
| Topología actual/futura | Confirmado por el propietario | Cortex, captura Facebook, Spotify y OBS corren hoy en la PC principal; la segunda PC será una notebook solo para control remoto. | Acordar acceso privado y permisos de la notebook al panel; no necesita backend/agente local. |
| Falla observada | Reportada por el propietario | En ocasiones el servidor Control Cortex se cierra o requiere reinicio; recuperación actual: volver a ejecutar `Iniciar Control Cortex.bat`. | El motivo y frecuencia no están diagnosticados; el nuevo núcleo necesita detección de salida, recuperación supervisada y registro del motivo. |
| Rendimiento/estabilidad | No verificado | No se corrieron mediciones ni se observaron logs. | No hay valores base para RAM, CPU, latencia o reconexión. |
| Requisitos futuros | Parcial | El plan y documentación describen panel remoto, reconstrucción desde cero y controles. | Prioridad de funciones y alcance final deben confirmarse con el propietario. |
| Seguridad | Parcial | AGENTS/docs indican controles y límites; se localizaron superficies LAN/OBS. | No se evaluaron firewall, permisos reales ni configuración activa. Revisar especialmente la operación de lotería por WebSocket y su alcance de red. |
| Lenguaje | Go elegido y prototipo del primer corte probado | Go 1.27.4, pruebas automatizadas y smoke test del ejecutable. | Sin medición comparativa de rendimiento o estabilidad del producto completo. |
| Aprobación y transición | Primer corte local autorizado y realizado; el resto sigue por aprobar por fases. | Decisiones posteriores del propietario e historial vigente en la tabla final. | No usar datos/cuentas reales ni reemplazar el sistema actual sin autorización específica. |

## Preguntas de operación que siguen abiertas

El propietario confirmó que usa todos los módulos salvo SocialStream Ninja, pide excluir SSN y simplificar/unificar los demás. Cortex, captura Facebook, Spotify y OBS están en una PC; la notebook futura será solo para control remoto desde el panel central, sin OBS ni agente local. Frecuencia por módulo y pasos exactos aún pendientes. No se debe clasificar como prescindible ninguna capacidad restante.

El propietario aclaró que hoy debe abrir módulos uno por uno y algunos se desconectan. “Unificar” significa operar desde un Cortex central, sin iniciar ni vigilar módulos por separado; OBS queda como aplicación principal aparte. También quiere validar comentarios de los vivos de sus dos cuentas de Facebook, destacar manualmente desde un historial que se actualiza, controlar Rulo y volver a lanzar bolilleros ya comprados, cerrar con guardado confiable y recuperar fallas automáticamente o con un botón/comando. La extensión actual de navegador debe quedar integrada o administrada por Cortex.

Sobre OBS, el propietario indicó revisar la carpeta local del perfil principal. La revisión confirma los tres archivos del perfil, pero no contiene colecciones de escenas. Queda pendiente una exportación de la colección realmente usada o una explicación equivalente de escenas/fuentes y casos especiales. La falta de exportación no impide documentar la propuesta provisional, pero sí deja incompleta la matriz OBS.

## Riesgos/prioridades a revisar

1. **Complejidad de implementación:** el propietario confirma que todos los módulos salvo SocialStream Ninja se usan. Hay que consolidar su funcionamiento en menos procesos, conservando cada capacidad y sin meter herramientas externas al núcleo si necesitan ciclo de vida propio.
2. **Desconocimiento de escenas y recursos OBS:** no se pueden vincular URLs Browser Source, animaciones, hotkeys, transiciones ni perfiles con funciones concretas.
3. **Datos personales y continuidad:** la SQLite y los archivos contienen historial/personas; la reconstrucción no debe borrar ni importar datos a ciegas.
4. **Seguridad de acceso remoto:** el usuario desea uso LAN/Tailscale sin login propio. El control depende de que la red/firewall limite dispositivos; no publicar HTTP administrativo ni OBS WebSocket.
5. **Operación sobre lotería:** documentación del proyecto advierte que un WebSocket de lotería en puerto 688 puede aceptar acciones de clientes que llegan al puerto. Confirmar si sigue vigente y limitar acceso en la nueva arquitectura.
6. **Facebook DOM:** cambios del sitio pueden romper captura aunque Cortex esté saludable. Diseñar indicadores de latido y estado por pestaña.
7. **Confirmación de Spotify:** no comunicar éxito por una respuesta HTTP aceptada; confirmar pista y dispositivo según regla existente si esa función se conserva.
8. **Datos/cobros externos:** los módulos de premios y correcciones están en alcance por confirmación del propietario. Mantener confirmación manual, idempotencia y auditoría.
9. **Estabilidad:** el primer corte confirma que el supervisor puede recuperar el servidor de prueba, no demuestra que Go evite fallos ni mide rendimiento del sistema completo.

## Decisiones vigentes y autoridad

| Tema | Estado actual | Quién puede cambiar/aprobar |
|---|---|---|
| Reconstruir el software de cero y no portar código | Instrucción confirmada por el propietario. | Propietario. |
| Carpeta documental IA 1 | Asignada por el propietario. | Propietario. |
| Implementación | Autorizado solo el primer corte nuevo y local, que ya fue probado; no incluye el Cortex actual ni cuentas/escenas reales. | Propietario decide cada siguiente fase. |
| Lenguaje principal Go | Elegido por el propietario e implementado en el primer corte. | Mantener Go salvo que una limitación concreta y comprobada cambie la decisión. |
| Arquitectura central | Aceptada para el primer corte: panel local y servidor interno gestionado por supervisor. | Confirmar ajustes de empaquetado/recuperación después de pruebas más largas. |
| Importar/conservar datos antiguos | No decidido; no se asume. | Propietario. |
| Retirar funciones | Excluir SocialStream Ninja; conservar las demás capacidades por confirmación del propietario. | Propietario puede cambiar el alcance. |
| Reemplazar OBS/Cortex actual | No autorizado. | Propietario después de aceptación real. |




