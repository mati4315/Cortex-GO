# Cortex Go — Índice del proyecto

**Fecha de actualización:** 12 de noviembre de 2026 (Australia/Brisbane)  
**Proyecto nuevo:** Cortex Go  
**Estado general:** Go fue elegido por el propietario; el primer corte local está implementado, compilado y probado. El proyecto se crea desde cero y no porta código del Cortex anterior. SocialStream Ninja queda excluido; los demás módulos continúan en alcance.  
**Límite vigente:** el permiso cubre el primer corte local. No se conectaron cuentas reales, no se importaron datos ni se modificaron el Cortex anterior o las escenas de OBS.

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

## Estado de los documentos

| Documento | Estado | Alcance |
|---|---|---|
| 00-INDICE.md | Actualizado | Estado vigente, cobertura y orden de lectura. |
| 01-AUDITORIA-CORTEX-ACTUAL.md | Parcial | Estructura, componentes, datos documentados e integraciones. |
| 02-AUDITORIA-OBS.md | Parcial | OBS 33.0.1 y el perfil indicado; no se encontraron escenas. |
| 03-INVENTARIO-FUNCIONAL-Y-MATRIZ.md | Parcial, revisado | Incluye alcance sin SSN, hotkeys OBS y UpCloud; frecuencia/PC pendiente. |
| 04-REQUISITOS-Y-DECISION-TECNOLOGICA.md | Decisión Go registrada | Go elegido; primer corte probado, producto completo aún sin medir. |
| 05-ARQUITECTURA-Y-PLAN-POR-FASES.md | Estado de fases actualizado | Primer corte implementado; fases reales pendientes. |
| 06-ESTADO-Y-PENDIENTES.md | Actualizado | Decisiones, pendientes y recomendaciones de continuación. |
| 07-ORGANIZACION-PREVIA-Y-DIAGRAMAS.md | Propuesta revisada | Agrupación funcional, diagramas Mermaid y estrategias previas al código. |
| 08-TRASPASO-ENTRE-IA.md | Histórico, actualizado | Entrega inicial con enlace al estado vigente. |
| 09-DICTAMEN-IA2.md | Revisión histórica | Dictamen previo al prototipo; incluye aviso de vigencia. |
| 10-FLUJO-OPERATIVO-REAL.md | Requisitos del propietario | Inicio mínimo, chequeo de dos cuentas, controles manuales, cierre y recuperación; referencias visuales. |
| 11-PRIMER-CORTE-PRUEBA-TECNICA.md | Implementado y probado | Supervisor, panel local, feeds sintéticos, SQLite y OBS de solo lectura. |
| 12-SINCRONIZACION-ENTRE-PC.md | Flujo público registrado; flujo privado pendiente de implementar | GitHub para código limpio; Syncthing propuesto para una copia privada cifrada y optativa. |

## Proyecto y publicación

- El proyecto se reconstruye desde cero en Go. Las rutas de las instalaciones locales se omiten de esta copia pública.
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
| Windows/operación | No verificado | No se consultaron procesos ni tareas actuales. | Dependencias, arranque, dispositivos, firewall y roles de PCs. |
| Integraciones | Parcial | Documentación/código identifica Spotify, Facebook, OBS, Cdelu/Firebase, IAFAS/MySQL, IA y red privada. | No se probaron conexiones ni recuperación. |
| Datos/persistencia | Parcial | Esquemas/manuales examinados, datos productivos no abiertos. | Decidir si se conservará o importará algún dato. |
| Alcance de módulos | Confirmado por el propietario | Usa todos salvo SocialStream Ninja; simplificar y unificar los demás. | Frecuencia y PC responsable por flujo pendientes. |
| SocialStream Ninja | Excluido | El propietario confirma que no lo usa y pide eliminarlo del diseño nuevo. | Revisar dependencias heredadas antes de desarrollo. |
| Rendimiento real | No verificado | No se midió. | Mediciones comparables solo si el propietario aprueba la prueba. |
| Lenguaje | Go elegido por el propietario para el núcleo | Recomendación IA 2; no hay mediciones de estabilidad. El panel/extensión se reescriben con tecnologías de navegador. | Validar en un primer corte el supervisor, recuperación, captura multicuenta y empaquetado. |
| Implementación/reemplazo | No autorizado | No se inició código nuevo; este trabajo solo actualizó Markdown. | Decisión expresa del propietario tras revisar pendientes y dictamen IA 2. |

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
| 2026-11-12 | Confirmado que la segunda PC será una notebook usada solo como control remoto. | No necesita OBS, backend ni agente local; usará el panel central de Cortex. |
| 2026-11-12 | Go queda elegido para el núcleo/backend. | El propietario pregunta y acepta la recomendación IA 2; la elección de lenguaje no sustituye validar recuperación del proceso. |
| 2026-11-12 | Registrada la incidencia del servidor central y el reinicio manual actual. | El primer corte Go debe incluir un supervisor que sobreviva al cierre del backend y documente la causa. |
| 2026-11-12 | Creado el esqueleto Go en la raíz y compilado `bin/CortexGo.exe`; instalado Go 1.27.4. | Inicio del proyecto nuevo; aún no hay módulos ni conexiones. |
| 2026-11-12 | Propuesto el primer corte técnico de Go con criterios de aceptación. | Validar recuperación y operación central antes de construir los módulos completos. |
| 2026-11-12 | Implementado el primer corte: supervisor, panel local, feeds de prueba, SQLite sintética y sesión OBS de solo lectura; pruebas automatizadas añadidas. | El proceso central se mantiene separado del servidor supervisado; Facebook real y módulos de directo siguen fuera del corte. |
| 2026-11-12 | Actualizados los estados de arquitectura, decisiones y traspaso para reflejar la autorización y la prueba del primer corte. | Las instrucciones antiguas de IA 1/IA 2 quedan como antecedente; prevalecen los documentos 05, 06 y 11 vigentes. |





