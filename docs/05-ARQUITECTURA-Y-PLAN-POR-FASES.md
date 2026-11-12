# Arquitectura propuesta y plan por fases

**Fecha:** 12 de noviembre de 2026 (Australia/Brisbane)  
**Estado:** el propietario eligió Go y autorizó implementar el primer corte local, que ya está compilado y probado. Las fases de integración real siguen pendientes.  
**Base:** alcance de módulos, flujo principal y topología de PCs confirmados; inventario fino de escenas OBS y algunos comportamientos por módulo siguen pendientes.

## Principio de reconstrucción

Crear un diseño y un código nuevos desde cero, preservando la capacidad funcional de todos los módulos salvo SocialStream Ninja, que el propietario confirmó que no usa y pide excluir. La meta es simplificar y unificar las capacidades restantes. No portar JavaScript, convertir archivos uno por uno, ni reproducir la estructura de carpetas como requisito. La versión actual sigue como referencia consultable y operativa mientras se construye. Esta fase no toca el repositorio viejo.

No se decide todavía si se conservarán los datos históricos. La propuesta predeterminada para empezar de cero es un esquema SQLite nuevo y vacío, sin importación automática. Si el propietario pide conservar ciertos datos, acordar por separado cuáles y diseñar una importación selectiva, verificable y aprobada. Nunca reemplazar ni borrar la SQLite actual por iniciativa de IA.

El objetivo operativo confirmado es iniciar Cortex una vez, operar todos sus módulos desde ahí y mantener OBS como la otra aplicación principal; no tener que lanzar y vigilar cada módulo por separado. La captura actual usa una extensión de navegador, así que Cortex puede tener que administrar ese auxiliar internamente. Ver [flujo operativo](10-FLUJO-OPERATIVO-REAL.md).

## Arquitectura mínima recomendada

**Monolito modular como línea base:** un único Cortex central, normalmente con un proceso principal Go y un panel común. Los módulos son responsabilidades internas, no lanzadores separados. Si Facebook u otra integración necesita un proceso auxiliar, Cortex lo inicia y supervisa; se separa solo por una razón técnica concreta. Un único Cortex mantiene la configuración y los datos; los navegadores remotos abren el panel por LAN/Tailscale. No se crea un sistema de plugins descargables ni un bus de mensajes distribuido.

| Límite lógico propuesto | Responsabilidad |
|---|---|
| Supervisor del proceso principal | Un componente de Windows independiente del servidor Go debe detectar que Cortex terminó y poder iniciarlo de nuevo con espera/reintentos limitados y registro del motivo. El mismo proceso que se cerró no puede recuperarse a sí mismo. Puede ser una política de servicio/Windows o un lanzador-supervisor oculto; se decidirá en el primer corte. |
| Núcleo y ciclo de vida | Un punto de inicio; arranque coordinado, estado de salud, reconexión, apagado y recuperación de módulos. Ningún módulo requiere lanzador manual propio. |
| Panel web y control remoto | Estado, configuración, diagnóstico y operaciones autorizadas; mismas APIs desde la PC principal o remota. |
| OBS | Cliente WebSocket con reconexión, estado de conexión, capacidades disponibles y confirmación de operaciones. |
| Entrada y eventos | Adaptadores para las fuentes que el propietario confirme; normalización, deduplicación y traza técnica mínima. |
| Reglas y acciones | Reglas explícitas para Spotify, chat, overlays y acciones de OBS; evita que una IA declare éxito sin evidencia. |
| Datos | Repositorios SQLite, esquema versionado, retención, respaldos y restauración. Una capa es suficiente mientras no se pruebe que hacen falta varias bases. |
| Medios | Catálogo/rutas, comprobación de disponibilidad y cache/miniaturas solo si la operación real lo necesita. |
| Integraciones externas | Adaptadores para Spotify, Facebook (captura desde navegador), Cdelu/Firebase, proveedor IA y otros servicios que el dueño acepte. El relay MediaMTX de UpCloud se mantiene como sistema externo con su ciclo de vida propio; Cortex nuevo debe documentar cómo preparar y comprobar el envío OBS sin absorber sus credenciales en el núcleo. |
| Supervisión externa | Solo para programas que deban seguir en proceso aparte por licencia, aislamiento, drivers o independencia operativa demostrada. No se crean procesos extra por herencia. |

Dentro del proceso, usar llamadas explícitas para solicitar y confirmar acciones; emitir eventos internos para comunicar hechos a varios componentes. Definir tamaño máximo de colas, política de reintentos y cancelación, evento repetido y acción idempotente. Persistir solo lo que deba sobrevivir a reinicios.

## OBS y topología de PCs

El propietario confirma que actualmente Cortex, captura de Facebook, Spotify y OBS están en la misma PC. Esa es la topología inicial; no hace falta separar Trigger Studio para lograr una instalación más unificada.

La segunda PC futura será una notebook de control remoto; no ejecutará OBS ni módulos de Cortex. Abrirá por LAN/Tailscale el mismo panel central, donde Trigger Studio será una sección más. No necesita backend, instalación de Trigger Studio ni agente local. OBS y la biblioteca quedan en la PC principal. Solo reconsiderar un agente si el propietario cambia el rol de la notebook en el futuro.

El Trigger Studio vigente documenta que su panel y WebSocket propios no tienen autenticación de usuario. No exponer ese servicio tal cual a otra PC o a Internet; al reconstruirlo, integrar los controles en Cortex con acceso remoto y autorización definidos. Confirmar el medio almacenado y la ubicación de su biblioteca antes de cerrar la topología.

Si OBS WebSocket cubre las acciones requeridas, mantener Cortex fuera del proceso OBS. No escribir un plugin C++ durante la reconstrucción salvo que una limitación reproducible de WebSocket lo justifique y el dueño apruebe esa excepción.

El deseo documentado es usar el panel por LAN o Tailscale sin cuentas adicionales. Eso es compatible con no mostrar un login propio solo si el acceso queda restringido por firewall y dispositivos/red confiables. No publicar acciones de OBS en Internet ni exponer el servicio WebSocket directamente. Acordar qué operaciones remotas pueden cambiar escenas, reproducir medios, activar música o conceder premios.

## Persistencia y privacidad

Separar la base de Cortex de preferencias visuales y de secretos. Guardar claves/tokens del lado del servidor, fuera del panel, logs, backups compartibles y control de versiones. Las trazas de eventos tienen que omitir texto de comentarios, nombres, IDs personales, URL de perfil y tokens salvo que un requisito aprobado lo justifique con controles.

La base nueva debe tener esquema versionado, transacciones para cambios que requieren consistencia y proceso de backup/restore ensayado con datos sintéticos antes de guardar datos reales. El formato interno no tiene por qué copiar las tablas existentes; si el propietario pide conservar historia, documentar campo a campo la importación antes de ejecutarla.

Mantener por diseño los límites seguros ya documentados si se aceptan esas funciones: identidad estable en lugar de coincidencia solo por nombre; pruebas aisladas de datos reales; asignación manual de premios; consulta de lotería de solo lectura en simulación; aprobación humana para cambios de personalidad/reglas; no enviar identidades privadas al proveedor IA.

## Plan por fases: estado actual

| Fase | Entrega verificable | Estado | Condición para avanzar |
|---|---|---|---|
| 0. Decisiones | Alcance principal, PCs, flujo operativo, Go y reconstrucción sin migración. | Completada en lo esencial; faltan detalles por módulo y exportación segura de escenas. | Completar decisiones concretas al entrar en cada módulo. |
| 1. Primer corte técnico en Go | [Panel central, supervisor, dos feeds de prueba, OBS de solo lectura y SQLite sintética](11-PRIMER-CORTE-PRUEBA-TECNICA.md). | Implementado, compilado y probado; sesión larga aún pendiente. | Aceptación visual del propietario y pruebas prolongadas/reintentos. |
| 2. Fundaciones | Proyecto nuevo, configuración local, logs, panel mínimo, cierre ordenado y ejecutable. | Base implementada para el corte local; remoto, configuración final y almacenamiento de secretos no están listos. | Acordar experiencia del panel y manejo seguro de credenciales. |
| 3. OBS y supervisión | Health, conexión WebSocket, consulta de escena y acciones reversibles. | Health/reconexión de proceso y sesión OBS de lectura implementadas; escenas sin integrar. | Tener una exportación de escenas limpia y definir una prueba reversible. |
| 4. Datos | SQLite nueva, esquema versionado, respaldo y restauración. | Persistencia sintética implementada; versión de esquema y backup/restore pendientes. | Diseñar y ensayar respaldo/restauración antes de datos reales. |
| 5. Flujos priorizados | Capacidades elegidas en cortes verticales, una por vez. | No iniciado; SocialStream Ninja permanece excluido. | Empezar por validar captura independiente de ambas cuentas de Facebook. |
| 6. Seguridad y recuperación | Acceso remoto, almacenamiento de credenciales, reconexión, idempotencia y fallos visibles. | Primer control local limitado a loopback; almacenamiento seguro de secretos y notebook pendientes. | Definir autenticación/red privada antes de abrir acceso LAN. |
| 7. Aceptación con OBS | Configuración representativa, simulación, revisión visual y guía operativa. | Pendiente. | Revisión con el propietario y plan de reversión antes de uso en directo. |

Las fases 4–5 abarcan las funciones de todos los módulos salvo SocialStream Ninja. Antes de programar cada flujo, documentar su comportamiento exacto, frecuencia, PC responsable, errores y criterio de aceptación; la confirmación de alcance no reemplaza esa especificación.

## Criterios de aceptación que se deben acordar

- El usuario puede identificar estado y error del núcleo, OBS y cada integración sin leer logs técnicos.
- OBS se reconecta de forma controlada y la UI distingue “aceptó la solicitud” de “acción confirmada”.
- En la operación de Spotify, no se anuncia reproducción hasta confirmar canción, dispositivo y estado.
- Las pruebas no contaminan historia/participantes ni afectan acciones reales.
- Las operaciones que conceden premios o cambian configuraciones críticas son confirmadas, idempotentes y auditables.
- Los respaldos se pueden restaurar; el modo de fallo no daña el archivo vigente.
- Panel remoto solo responde en la red/dispositivos autorizados y no divulga credenciales.
- Rendimiento se compara con el mismo escenario, máquinas y carga; no se aceptan ahorros prometidos sin medición.
- Se prueban los flujos antes/durante/después del directo que el propietario confirme, incluidos fallos de conexión y acciones excepcionales.
- Con una sola acción de inicio, el panel identifica por separado el estado de captura y el último comentario de cada cuenta de Facebook antes de salir al aire.
- El cierre confirma que la base no tiene escrituras pendientes; los comandos de recuperación distinguen reconectar, reiniciar un módulo y reiniciar Cortex completo.



