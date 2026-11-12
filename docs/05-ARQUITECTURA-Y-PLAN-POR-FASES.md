# Arquitectura propuesta y plan por fases

**Fecha:** 13 de noviembre de 2026 (Australia/Brisbane)
**Estado:** el propietario eligió Go y autorizó terminar la base operativa. El núcleo, la PWA y un prototipo externo de captura están implementados y desplegados en el equipo disponible. La PC habitual está apagada; la aceptación de Facebook real queda pendiente.
**Base:** alcance de módulos, flujo principal y topología de PCs confirmados; inventario fino de escenas OBS y algunos comportamientos por módulo siguen pendientes.

## Principio de reconstrucción

Crear un diseño y un código nuevos desde cero, preservando la capacidad funcional de todos los módulos salvo SocialStream Ninja, que el propietario confirmó que no usa y pide excluir. La meta es simplificar y unificar las capacidades restantes. No portar JavaScript, convertir archivos uno por uno, ni reproducir la estructura de carpetas como requisito. La versión actual sigue como referencia consultable y operativa mientras se construye. Esta fase no toca el repositorio viejo.

No se importan los datos históricos del Cortex anterior. La base nueva conserva nombres y textos de los comentarios de Facebook hasta que el usuario los borre manualmente. Borrar el historial activo no borra los respaldos existentes; estos tienen que eliminarse por separado. Nunca reemplazar ni borrar la SQLite del Cortex anterior.

El objetivo operativo confirmado es iniciar Cortex una vez, operar todos sus módulos desde ahí y mantener OBS como la otra aplicación principal; no tener que lanzar y vigilar cada módulo por separado. La captura actual usa una extensión de navegador, así que Cortex puede tener que administrar ese auxiliar internamente. Ver [flujo operativo](10-FLUJO-OPERATIVO-REAL.md).

## Arquitectura mínima recomendada

**Monolito modular como línea base:** un único Cortex central, normalmente con un proceso principal Go y un panel común. Los módulos son responsabilidades internas, no lanzadores separados. Si Facebook u otra integración necesita un proceso auxiliar, Cortex lo inicia y supervisa; se separa solo por una razón técnica concreta. Un único Cortex mantiene la configuración y los datos; los navegadores remotos abren el panel por LAN/Tailscale. No se crea un sistema de plugins descargables ni un bus de mensajes distribuido.

| Límite lógico propuesto | Responsabilidad |
|---|---|
| Supervisor del proceso principal | Lanzador oculto independiente del servidor Go; recupera hasta cinco cierres inesperados con esperas limitadas y registra los motivos. El panel habilita o deshabilita un acceso directo por usuario para iniciar Cortex al iniciar sesión; no se instala un servicio de Windows. |
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

El diseño mantiene una sola instancia activa y una sola base de datos. La instancia está desplegada temporalmente en el equipo disponible mientras la PC habitual está apagada; el acceso remoto usa Tailscale. Al volver a encender la PC habitual, cualquier traslado se hará con una copia verificada y sin dos instancias escribiendo a la vez.

El Trigger Studio vigente documenta que su panel y WebSocket propios no tienen autenticación de usuario. No exponer ese servicio tal cual a otra PC o a Internet; al reconstruirlo, integrar los controles en Cortex con acceso remoto y autorización definidos. Confirmar el medio almacenado y la ubicación de su biblioteca antes de cerrar la topología.

Si OBS WebSocket cubre las acciones requeridas, mantener Cortex fuera del proceso OBS. No escribir un plugin C++ durante la reconstrucción salvo que una limitación reproducible de WebSocket lo justifique y el dueño apruebe esa excepción.

El deseo documentado es usar el panel por LAN o Tailscale sin cuentas adicionales. Eso es compatible con no mostrar un login propio solo si el acceso queda restringido por firewall y dispositivos/red confiables. No publicar acciones de OBS en Internet ni exponer el servicio WebSocket directamente. Acordar qué operaciones remotas pueden cambiar escenas, reproducir medios, activar música o conceder premios.

## Persistencia y privacidad

Separar la base de Cortex de preferencias visuales y de secretos. Guardar claves/tokens del lado del servidor, fuera del panel, logs, backups compartibles y control de versiones. Las trazas de eventos tienen que omitir texto de comentarios, nombres, IDs personales, URL de perfil y tokens salvo que un requisito aprobado lo justifique con controles.

La base nueva ya tiene esquema versionado y una migración transaccional. El registro permite respaldar, validar y restaurar snapshots; esos pasos aún deben aceptarse en el escritorio antes de depender de ellos en producción. No se importan filas históricas del Cortex anterior. Los comentarios nuevos guardan nombre y texto hasta borrado manual en la base activa; los respaldos conservan su propio ciclo de vida y todavía no están cifrados.

Mantener por diseño los límites seguros ya documentados si se aceptan esas funciones: identidad estable en lugar de coincidencia solo por nombre; pruebas aisladas de datos reales; asignación manual de premios; consulta de lotería de solo lectura en simulación; aprobación humana para cambios de personalidad/reglas; no enviar identidades privadas al proveedor IA.

## Plan por fases: estado actual

| Fase | Entrega verificable | Estado | Condición para avanzar |
|---|---|---|---|
| 0. Decisiones | Alcance principal, PCs, flujo operativo, Go y reconstrucción sin migración. | Completada en lo esencial; faltan detalles por módulo y exportación segura de escenas. | Completar decisiones concretas al entrar en cada módulo. |
| 1. Primer corte técnico en Go | [Panel central, supervisor, dos feeds de prueba, OBS de solo lectura y SQLite inicial](11-PRIMER-CORTE-PRUEBA-TECNICA.md). | Implementado; la sesión larga y la aceptación en el escritorio siguen pendientes. | Desplegar en el equipo principal y completar validación operativa. |
| 2. Fundaciones | Proyecto nuevo, configuración local, logs, panel mínimo, cierre ordenado, PWA y ejecutable. | Implementado, publicado y desplegado; el inicio automático se controla desde el panel y no requiere un servicio. | Confirmar instalación/uso de PWA en los dispositivos previstos. |
| 3. OBS y supervisión | Health, conexión WebSocket, consulta de escena y acciones reversibles. | OBS sigue en solo lectura; lanzador recupera el proceso con hasta cinco reintentos. Escenas y acciones no integradas. | Exportar escenas limpias y definir una prueba reversible. |
| 4. Datos | SQLite nueva, esquema versionado, respaldo, restauración y retención manual. | Registro general v1, deduplicación, borrado de historial, listado/borrado de respaldos y backup/restore implementados. Se migró la base existente y se preservó un respaldo. Backups sin cifrar. | Probar restauración con datos ficticios y decidir custodia externa/cifrado. |
| 5. Flujos priorizados | Capacidades elegidas en cortes verticales, una por vez. | Prototipo Facebook externo conectado al API; acceso real, CEF y demás módulos pendientes. | Autenticar manualmente las dos cuentas en escritorio y verificar captura real. |
| 6. Seguridad y recuperación | Acceso remoto, almacenamiento de credenciales, reconexión, idempotencia y fallos visibles. | API local limitada a loopback; Tailscale Serve publica el panel solo dentro del tailnet. Perfiles locales separados; gestión de secretos y sincronización privada pendientes. | Mantener el acceso limitado al tailnet y validar el host habitual cuando vuelva a estar disponible. |
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



