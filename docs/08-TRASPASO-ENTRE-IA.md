# Traspaso de IA 1 a IA 2 — Cortex 2.0

**Fecha:** 12 de noviembre de 2026 (Australia/Brisbane)  
**Tipo de traspaso:** provisional; auditoría incompleta por evidencia operativa pendiente.  
**Implementación al momento de este traspaso:** no iniciada ni autorizada. Este dato es histórico.

**Resultado posterior:** IA 2 dejó su dictamen condicional en [09-DICTAMEN-IA2.md](09-DICTAMEN-IA2.md). Esta revisión fue una segunda pasada de la misma IA en el mismo hilo, no una validación independiente.

**Actualización vigente:** el propietario eligió Go y autorizó el primer corte local, que ya está implementado y probado. No se reemplazó ni modificó el Cortex actual. Para el estado y los siguientes pasos, prevalecen [06-ESTADO-Y-PENDIENTES.md](06-ESTADO-Y-PENDIENTES.md) y [11-PRIMER-CORTE-PRUEBA-TECNICA.md](11-PRIMER-CORTE-PRUEBA-TECNICA.md).

## Resumen

El propietario asignó el rol IA 1 y confirmó que quiere **crear Cortex desde cero en Go, sin migrar el código del proyecto actual**. Las rutas locales de la instalación anterior y de OBS se omiten de esta copia pública.

La versión actual tiene varios servicios Node.js y una aplicación central, overlays/paneles web, una extensión de navegador, bases de datos SQLite, servicios externos y control de OBS por WebSocket. El propietario confirmó que usa todos los módulos salvo SocialStream Ninja; ese módulo y sus dependencias quedan excluidos, y las demás capacidades deben simplificarse/unificarse sin perderse. Falta especificar frecuencia y reparto entre PCs.

Se examinó una carpeta local de perfil de OBS. Allí hay basic.ini, service.json y streamEncoder.json; no hay colección de escenas. Se confirmó versión OBS 33.0.1 y ajustes de salida en el perfil, pero no uso en vivo, fuentes, escenas ni dependencias multimedia.

**Decisión tecnológica propuesta:** Go para el backend/núcleo Windows, confianza media, no aprobada. Mantener UI web y la extensión en tecnologías de navegador reescritas desde cero. No se encontró necesidad verificada de un plugin C++ para OBS.

## Documentos para revisar

1. [Índice y cobertura](00-INDICE.md)
2. [Auditoría del Cortex vigente](01-AUDITORIA-CORTEX-ACTUAL.md)
3. [Auditoría de OBS](02-AUDITORIA-OBS.md)
4. [Inventario funcional/matriz Cortex–OBS](03-INVENTARIO-FUNCIONAL-Y-MATRIZ.md)
5. [Requisitos y decisión tecnológica](04-REQUISITOS-Y-DECISION-TECNOLOGICA.md)
6. [Arquitectura y fases propuestas](05-ARQUITECTURA-Y-PLAN-POR-FASES.md)
7. [Organización previa y diagramas](07-ORGANIZACION-PREVIA-Y-DIAGRAMAS.md)
8. [Estado, riesgos y preguntas](06-ESTADO-Y-PENDIENTES.md)

## Qué está verificado y qué no

**Verificado por lectura local:** el árbol/documentación del repositorio y sus manifiestos; múltiples componentes; rutas de servicios documentadas; un archivo modificado en el repositorio vigente; OBS 33.0.1; lista de DLL de plugins presentes; tres archivos de perfil y su configuración básica.

**No verificado:** operación real de Windows/OBS, colecciones y fuentes de escenas, conexiones activas, frecuencia de funciones, costos de recursos, datos reales, integraciones autenticadas, respaldo/restauración o casos excepcionales. No se midieron RAM/CPU/latencia. No se leyeron claves ni filas de bases de datos.

El propietario confirmó el alcance de módulos, pero aún falta frecuencia habitual/ocasional, secuencia de trabajo y PC responsable. No declarar terminada la auditoría funcional.

## Recomendación y condiciones

Go encaja como núcleo de servicios persistentes HTTP/WebSocket y coordinación, sin que la evidencia demuestre ventaja de rendimiento. C#/.NET sigue como competidor fuerte en Windows; Node sería la alternativa de menor riesgo, aunque no coincide con el deseo de elegir otro lenguaje. Rust y PHP tienen usos posibles, pero no presentan una ventaja comprobada para este caso. C++ solo si una limitación real exige plugin nativo de OBS.

IA 2 revisó la recomendación y dejó condiciones en el dictamen 09. Antes de implementación, el propietario tendría que completar alcance y aprobar una prueba acotada en Windows, comparando Go y C#/.NET con SQLite sintética y conexión OBS de ensayo. No usar datos reales ni alterar la instalación de OBS para el prototipo.

## Revisión solicitada a IA 2

- Comprobar que todas las capacidades, excepto SocialStream Ninja, aparecen en el alcance y que el nuevo diseño las agrupa sin conservar servicios innecesarios.
- Verificar la interpretación de las políticas de seguridad/aislamiento de Facebook, Spotify, Rulo, Ruleta y premios.
- Cuestionar la recomendación Go frente a C#/.NET y el costo de conservar una extensión JavaScript; revisar si Java/Kotlin o Python añaden una ventaja concreta bajo los requisitos confirmados.
- Evaluar la matriz OBS; pedir colecciones/fuentes o mantener explícitamente la limitación.
- Documentar frecuencia, eventos ocasionales y reparto de PCs; cualquier retiro de capacidad requiere nueva confirmación del propietario. Simplificar la estructura sin perder el alcance que ya confirmó.
- Revisar el plan de datos: la reconstrucción es nueva; no hay autorización ni decisión para importar datos existentes.
- Comprobar que los pasos de aceptación incluyen conexión/desconexión, restauración de base, aislamiento, seguridad LAN y validación visual.
- Emitir recomendación favorable, condicional o correcciones; no autorizar desarrollo por el propietario.

## Límites del trabajo de IA 1

IA 1 creó únicamente documentos Markdown en la carpeta asignada. No modificó el código fuente de Cortex, la instalación/configuración de OBS ni datos productivos; no instaló dependencias, no inició servicios, no realizó pruebas y no comenzó la reconstrucción. No inspeccionó el contenido del archivo modificado de Git ni credenciales.






