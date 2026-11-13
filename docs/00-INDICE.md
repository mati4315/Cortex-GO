# Cortex Go — Índice del proyecto

**Fecha de actualización:** 13 de noviembre de 2026 (Australia/Brisbane)
**Proyecto nuevo:** Cortex Go  
**Estado general:** la base Go está publicada y desplegada en el equipo disponible, con PWA, lanzador de recuperación, SQLite versionada y API de eventos. Bienvenidos y Banner y Luces están integrados; la aceptación de OBS requiere la computadora que ejecuta OBS. La PC habitual está apagada; su eventual traslado y la aceptación de Facebook real también esperan que vuelva a estar disponible. SocialStream Ninja queda excluido; los demás módulos continúan en alcance.
**Límite vigente:** no se importaron configuraciones privadas, perfiles, sesiones ni medios personales del Cortex anterior y no se modificó esa instalación. Las sesiones reales de Facebook requieren autenticación manual. Luces genera un overlay visual y no controla hardware DMX. El cambio de escena de Bienvenidos y Banner es local y opcional; empieza desactivado.

## Orden de lectura

1. Este índice y la cobertura.
2. [01 — Auditoría del Cortex vigente](01-AUDITORIA-CORTEX-ACTUAL.md)
3. [02 — Auditoría de OBS](02-AUDITORIA-OBS.md)
4. [03 — Inventario funcional y matriz Cortex–OBS](03-INVENTARIO-FUNCIONAL-Y-MATRIZ.md)
5. [04 — Requisitos y decisión tecnológica](04-REQUISITOS-Y-DECISION-TECNOLOGICA.md)
6. [05 — Arquitectura y plan por fases](05-ARQUITECTURA-Y-PLAN-POR-FASES.md)
7. [06 — Estado y decisiones pendientes](06-ESTADO-Y-PENDIENTES.md)
8. [07 — Organización previa y diagramas](07-ORGANIZACION-PREVIA-Y-DIAGRAMAS.md)
9. [08 — Traspaso provisional para IA 2](08-TRASPASO-ENTRE-IA.md)
10. [09 — Dictamen de revisión IA 2](09-DICTAMEN-IA2.md)
11. [10 — Flujo operativo real y referencias visuales](10-FLUJO-OPERATIVO-REAL.md)
12. [11 — Primer corte técnico](11-PRIMER-CORTE-PRUEBA-TECNICA.md)
13. [12 — Sincronización entre la PC y la notebook](12-SINCRONIZACION-ENTRE-PC.md)
14. [13 — Guía para capturar comentarios de dos directos de Facebook](13-GUIA-CAPTURA-COMENTARIOS-FACEBOOK.md)
15. [14 — Estado de la base operativa](14-BASE-OPERATIVA.md)
16. [15 — Módulo Bienvenidos y Banner](15-MODULO-BIENVENIDOS-Y-BANNER.md)
17. [16 — Módulo Luces](16-MODULO-LUCES.md)

## Estado de los documentos

| Documento | Estado | Alcance |
|---|---|---|
| 00-INDICE.md | Actualizado | Estado vigente, cobertura y orden de lectura. |
| 01-AUDITORIA-CORTEX-ACTUAL.md | Parcial | Estructura, componentes, datos documentados e integraciones. |
| 02-AUDITORIA-OBS.md | Parcial | OBS 33.0.1 y el perfil indicado; no se encontraron escenas. |
| 03-INVENTARIO-FUNCIONAL-Y-MATRIZ.md | Parcial, revisado | Incluye alcance sin SSN, hotkeys OBS y UpCloud; frecuencia/PC pendiente. |
| 04-REQUISITOS-Y-DECISION-TECNOLOGICA.md | Decisión Go registrada | Go elegido; código de la base actualizado, producto completo aún sin medir. |
| 05-ARQUITECTURA-Y-PLAN-POR-FASES.md | Actualizado al estado de la base | PWA, lanzador, SQLite v1 y prototipo Facebook; despliegue en el equipo disponible. |
| 06-ESTADO-Y-PENDIENTES.md | Actualizado | Decisiones y validaciones que requieren acceso a Facebook o al equipo habitual. |
| 07-ORGANIZACION-PREVIA-Y-DIAGRAMAS.md | Propuesta revisada | Agrupación funcional, diagramas Mermaid y estrategias previas al código. |
| 08-TRASPASO-ENTRE-IA.md | Histórico, actualizado | Entrega inicial con enlace al estado vigente. |
| 09-DICTAMEN-IA2.md | Revisión histórica | Dictamen previo al prototipo; incluye aviso de vigencia. |
| 10-FLUJO-OPERATIVO-REAL.md | Requisitos del propietario | Inicio mínimo, chequeo de dos cuentas, controles manuales, cierre y recuperación; referencias visuales. |
| 11-PRIMER-CORTE-PRUEBA-TECNICA.md | Histórico del primer corte | Supervisor interno, feeds sintéticos, SQLite inicial y OBS de solo lectura. |
| 12-SINCRONIZACION-ENTRE-PC.md | Actualizado al host disponible | Una sola instancia mantiene la base activa; acceso remoto por Tailscale. No sincronizar SQLite activa; copias cifradas y Syncthing siguen pendientes y optativos. |
| 13-GUIA-CAPTURA-COMENTARIOS-FACEBOOK.md | Captura prototipada; validación manual pendiente | El [prototipo de dos perfiles](../experiments/facebook-browser-poc/README.md) envía eventos nuevos al API local; faltan login/2FA y aceptación en la PC principal. |
| 14-BASE-OPERATIVA.md | Base publicada y desplegada | Resume el despliegue y las comprobaciones realizadas; la autenticación de Facebook y aceptación de la PC habitual quedan pendientes. |
| 15-MODULO-BIENVENIDOS-Y-BANNER.md | Integrado en Go | URLs, controles, persistencia SQLite v2, OBS local opcional y validación visual pendiente. |
| 16-MODULO-LUCES.md | Integrado en Go | Efectos Canvas, seis presets, configuración SQLite y sincronización en tiempo real; aceptación visual pendiente en OBS. |

## Proyecto y publicación

- El servidor y persistencia se reconstruyen en Go. Para preservar la fidelidad, Bienvenidos y Banner adapta hojas de estilo, scripts de overlay y recursos gráficos; Luces conserva sus pantallas y renderizado Canvas con una API Go. `.env`, configuraciones, cuentas, sesiones y medios personales quedan fuera. Las rutas de las instalaciones locales se omiten de esta copia pública.
- El propietario decidió mantener público el remoto de GitHub; se publican solo código y documentación revisados, sin bases, claves, datos privados ni capturas personales.
- El primer commit limpio ya se publicó en la rama `main`. Las capturas y rutas locales quedan fuera del repositorio. Revisar [el plan de sincronización](12-SINCRONIZACION-ENTRE-PC.md).
- El Cortex anterior permanece separado y sin cambios.

## Matriz de cobertura

| Área | Estado | Evidencia/alcance | Pendiente |
|---|---|---|---|
| Guía de roles y criterios | Revisada en segunda pasada | Plan Cortex IA 1 versión 4.4, leído completo; dictamen IA 2 en este hilo. | La revisión no fue independiente porque la realizó la misma IA. |
| Código/dependencias Cortex | Parcial | Estructura, manifiestos y documentación; búsquedas de rutas y bases. | Revisar detalles funcionales y relaciones de todos los módulos. |
| OBS | Parcial | OBS 33.0.1, perfil de salida y DLL presentes. | Colecciones, fuentes, scripts de usuario, WebSocket activo y uso real. |
| Flujo real | Parcial, ampliado por el propietario | Pasos antes/durante/después, recuperación deseada, PC principal y notebook remota. | Escenas/fuentes exactas, semántica de acciones manuales y criterio de datos por guardar. |
| Windows/operación | Parcial | Go compila para Windows; lanzador y PWA implementados. La PC principal aparece desconectada. | Desplegar, validar recuperación, Tailscale Serve y uso desde la notebook. |
| Integraciones | Parcial | Documentación/código identifica Spotify, Facebook, OBS, Cdelu/Firebase, IAFAS/MySQL, IA y red privada. | No se probaron conexiones ni recuperación. |
| Datos/persistencia | Base implementada en código | SQLite v1, eventos generales, deduplicación, backup/restore y borrado manual; no se importan datos antiguos. | Despliegue y validación en el escritorio; respaldos aún sin cifrar. |
| Alcance de módulos | Confirmado por el propietario | Usa todos salvo SocialStream Ninja; simplificar y unificar los demás. | Frecuencia y PC responsable por flujo pendientes. |
| SocialStream Ninja | Excluido | El propietario confirma que no lo usa y pide eliminarlo del diseño nuevo. | Revisar dependencias heredadas antes de desarrollo. |
| Rendimiento real | No verificado | No se midió. | Mediciones comparables solo si el propietario aprueba la prueba. |
| Lenguaje | Go elegido por el propietario para el núcleo | Recomendación IA 2; no hay mediciones de estabilidad. El panel/extensión se reescriben con tecnologías de navegador. | Validar en un primer corte el supervisor, recuperación, captura multicuenta y empaquetado. |
| Implementación/reemplazo | Base nueva autorizada; reemplazo no realizado | Núcleo, SQLite v1, PWA y prototipo Facebook compilan; Cortex anterior y escenas OBS siguen intactos. | Despliegue y validación manual en la PC principal; login/2FA y captura real pendientes. |

## Criterio de evidencia

Las afirmaciones se marcan como **Confirmado**, **Probable**, **Hipótesis**, **Recomendación**, **Pendiente de validar** o **No verificado**. Las recomendaciones distinguen beneficio, riesgo y condición. No se copiaron secretos, tokens, claves, datos de personas ni filas de bases.

## Registro de cambios y decisiones

| Fecha | Cambio | Motivo |
|---|---|---|
| 2026-11-12 | Creado índice inicial y fijado el alcance documental. | Carpeta asignada; rol IA 1; reconstrucción desde cero. |
| 2026-11-12 | Añadidas auditorías, matriz funcional, recomendación tecnológica, arquitectura y traspaso provisional. | Inspección de solo lectura del Cortex y las rutas OBS indicadas. |
| 2026-11-12 | Confirmada ausencia de escenas en el perfil de OBS revisado. | La carpeta examinada contenía tres archivos del perfil, sin una colección de escenas. |
| 2026-11-12 | Actualizado el alcance por declaración del propietario; SocialStream Ninja queda excluido. | El propietario confirma que usa todos los demás módulos y quiere simplificarlos/unificarlos. |
| 2026-11-12 | Añadida comparación de lenguajes fuera de la lista inicial. | Evaluación documental de Java/Kotlin y Python; no aparece un candidato claramente superior a Go. |
| 2026-11-12 | Añadido dictamen IA 2, corregida una fila de OBS y afinados los diagramas y límites de la recomendación Go. | Revisión crítica documental; no hubo implementación ni prueba técnica. |
| 2026-11-12 | Añadido el flujo operativo real y referencias visuales aportadas por el propietario. | Inicio mínimo, validación de dos cuentas, acciones manuales, cierre y recuperación. |
| 2026-11-12 | Aclarado que “unificar” significa un único punto de inicio y supervisión, no necesariamente un solo proceso. | El propietario quiere evitar abrir y vigilar cada módulo por separado; los auxiliares deben quedar gestionados por Cortex. |
| 2026-11-12 | Confirmada la PC actual y recomendado integrar Trigger Studio al panel central para el acceso remoto futuro. | La segunda PC puede ser solo cliente de control; un agente separado depende de si allí se ejecutan OBS o recursos locales. |
| 2026-11-13 | Integrado el módulo Luces con configuración SQLite y eventos en tiempo real. | Portar los efectos, presets y controles del proyecto anterior sin su servidor Node ni su configuración privada. |
| 2026-11-12 | Confirmado que la segunda PC será una notebook usada solo como control remoto. | No necesita OBS, backend ni agente local; usará el panel central de Cortex. |
| 2026-11-12 | Go queda elegido para el núcleo/backend. | El propietario pregunta y acepta la recomendación IA 2; la elección de lenguaje no sustituye validar recuperación del proceso. |
| 2026-11-12 | Registrada la incidencia del servidor central y el reinicio manual actual. | El primer corte Go debe incluir un supervisor que sobreviva al cierre del backend y documente la causa. |
| 2026-11-12 | Creado el esqueleto Go en la raíz y compilado `bin/CortexGo.exe`; instalado Go 1.27.4. | Inicio del proyecto nuevo; aún no hay módulos ni conexiones. |
| 2026-11-12 | Propuesto el primer corte técnico de Go con criterios de aceptación. | Validar recuperación y operación central antes de construir los módulos completos. |
| 2026-11-12 | Implementado el primer corte: supervisor, panel local, feeds de prueba, SQLite sintética y sesión OBS de solo lectura; pruebas automatizadas añadidas. | El proceso central se mantiene separado del servidor supervisado; Facebook real y módulos de directo siguen fuera del corte. |
| 2026-11-12 | Actualizados los estados de arquitectura, decisiones y traspaso para reflejar la autorización y la prueba del primer corte. | Las instrucciones antiguas de IA 1/IA 2 quedan como antecedente; prevalecen los documentos 05, 06 y 11 vigentes. |
| 2026-11-12 | Corregida la guía de Facebook: se descarta la API oficial y se documenta la captura DOM como referencia del Cortex anterior. | Se revisaron la extensión, los perfiles separados por cuenta, la apertura de Live Producer y las señales de estado; la reconstrucción seguirá siendo desde cero. |
| 2026-11-12 | Preparado un prototipo aislado con dos perfiles Edge/Brave, DevTools local y observador DOM; compilación correcta. | Iniciar la prueba de sesiones separadas sin copiar la extensión vieja; faltan login/2FA y persistencia manual, y evaluar CEF antes de integrar al Cortex principal. |
| 2026-11-13 | Terminada la base en código: lanzador de recuperación, SQLite v1 con eventos y deduplicación, retención manual de historial y respaldos, métricas de captura, PWA y puente local para el prototipo de dos cuentas. | Dejar listas las fundaciones antes de continuar con los módulos; las cuentas reales no se validaron. |
| 2026-11-13 | Confirmado el escritorio como host principal y la retención de comentarios hasta borrado manual. | La PC sigue desconectada; la notebook actuará como cliente Tailscale y los respaldos tienen un ciclo de vida independiente. |
| 2026-11-13 | Documentado el estado real de la base y los pendientes de despliegue en documentos 05, 06, 12, 13 y 14. | Evitar presentar como validado lo que requiere acceso a la PC de escritorio o autenticación manual. |





