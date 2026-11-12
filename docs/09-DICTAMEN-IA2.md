# Dictamen de revisión — IA 2

**Fecha:** 12 de noviembre de 2026 (Australia/Brisbane)  
**Rol:** revisión crítica de la auditoría, el inventario, la arquitectura y la elección tecnológica.  
**Dictamen:** **favorable con condiciones para continuar la definición del producto; no listo para iniciar la reconstrucción ni para ratificar Go como decisión final.**  
**Alcance de esta revisión:** documentos y lectura estática del código disponible. No se ejecutó el sistema, no se probaron conexiones y no se hicieron cambios de código ni de OBS.

> **Vigencia:** este dictamen describe la revisión hecha antes de la implementación. Después, el propietario eligió Go y autorizó solo el primer corte local, registrado en [11-PRIMER-CORTE-PRUEBA-TECNICA.md](11-PRIMER-CORTE-PRUEBA-TECNICA.md). Este cambio no aprueba la reconstrucción completa ni el reemplazo del sistema actual.

## Límite de independencia

Esta revisión cumple el rol documental de IA 2 solicitado, pero la realizó la misma IA dentro del mismo hilo que produjo el trabajo de IA 1. Por ello sirve como una segunda pasada crítica y deja hallazgos concretos, pero **no equivale a una auditoría independiente**. El propietario conserva la autoridad sobre alcance, lenguaje y autorización de implementación.

## Veredicto por decisión

| Decisión | Dictamen IA 2 | Motivo |
|---|---|---|
| Reconstruir desde cero, sin portar código | Aceptar | Coincide con la instrucción expresa del propietario. El código actual es evidencia de comportamiento, no base para traducir o migrar. |
| Conservar las funciones de todos los módulos menos SocialStream Ninja | Aceptar como requisito | Es una decisión del propietario. Falta validar el inventario contra cada carpeta, recurso y flujo; no se debe retirar otra capacidad por frecuencia baja o por falta de escenas exportadas. |
| Excluir SocialStream Ninja | Aceptar | Excluir su servidor, extensión, relay, overrides e integración. Antes de borrar o reemplazar referencias en el sistema antiguo, identificar cualquier comportamiento que el propietario sí usa y asignarle otro dueño en el diseño nuevo. |
| Monolito modular como hipótesis inicial | Aceptar con condición | Es una buena forma de reducir procesos y paneles. Mantener separaciones de proceso cuando una dependencia, permisos, ciclo de vida o aislamiento de fallas lo justifiquen. No se ha probado que todo deba correr en un único proceso o equipo. |
| Go como lenguaje principal | Candidato preferido para una prueba; no aprobado | Encaja con un backend de servicios y red, pero la recomendación aún no tiene prototipo ni mediciones. C#/.NET continúa siendo un competidor fuerte para Windows. |
| Go para todo el software, incluida la interfaz/extensión | No aceptar tal como está formulado | El propio plan conserva HTML/CSS/JavaScript para panel y extensión. La propuesta real es backend Go más código nuevo de navegador. El propietario debe considerar esa división compatible con “otro lenguaje”; si exige una sola tecnología, se debe reabrir el alcance. |
| Base SQLite nueva | Aceptar como propuesta, pendiente de validación | Crear un archivo/esquema nuevo separado del actual es coherente con empezar de cero. No se decidió si el archivo histórico se conservará como archivo consultable, solo se respaldará o se dejará fuera de operación. |

## Hallazgos y correcciones aplicadas

1. **Función OBS omitida:** el Cortex actual contiene `POST /api/obs/trigger-hotkey` y llama `TriggerHotkeyByKeySequence`; el código describe hotkeys numéricas 1/2/3 para cambiar escenas. Se añadió a la matriz funcional como implementado en código, con escena, conexión y uso real aún no verificados.
2. **Secuencia de comentarios incompleta:** el diagrama ahora incluye explícitamente la evaluación de Rulo. El flujo de prueba se representa como simulación sin escritura persistente ni llamadas a Spotify/OBS; el comportamiento que realmente se quiera probar contra servicios debe definirse como acción separada y deliberada.
3. **Reintentos de Spotify:** se añadió la necesidad de reconciliar el estado cuando se pierde una respuesta después de emitir una orden. Reintentar a ciegas puede repetir una acción externa o anunciar éxito sin reproducción confirmada.
4. **SQLite en Go:** se explicitó que `database/sql` no trae drivers incluidos. La selección de driver, su licencia, dependencias transitivas, soporte Windows y comportamiento de concurrencia deben formar parte de la evaluación. `modernc.org/sqlite` es una opción sin CGO, no una decisión ya tomada.
5. **Flujo UpCloud ausente de la matriz:** el README documenta OBS → SRT cifrado → relay MediaMTX en VPS → dos destinos Facebook por RTMPS, además de scripts para claves, despliegue, estado y configuración OBS. La función queda incluida; operación en vivo, PC responsable, recuperación y rotación siguen pendientes. La URL SRT es sensible porque permite publicar y debe quedar fuera de logs/paneles compartibles. No se copiaron identificadores ni se leyeron claves. También se dejó pendiente clasificar los directorios de temas/recursos como activos o no ejecutables.
6. **Correcciones editoriales:** se arregló la fila concatenada del registro del índice y el salto de párrafo antes de la sección de interfaz.

## Condiciones antes de autorizar el proyecto nuevo

### A. Cerrar alcance y comportamiento

- Crear una tabla de trazabilidad por capacidad: carpeta/módulo actual → comportamiento observado en documentación/código → conservación deseada → dueño nuevo → criterio de aceptación. Marcar por separado las carpetas que sean assets, temas o documentación y no módulos ejecutables.
- Completar frecuencia, criticidad, orden de operación y PC responsable, especialmente para OBS, captura de Facebook, Spotify, Rulo, sorteos/premios, luces y overlays.
- Separar lo implementado hoy de ideas propuestas o capacidades solo descritas en documentos. La frase “uso todos los módulos” confirma alcance, pero no valida cada detalle ni conexión de los diagramas.
- Confirmar qué extensión de navegador se reconstruye y aceptar expresamente que el backend sea Go mientras las páginas y el adaptador de extensión sean código nuevo de navegador.

### B. Completar el diseño y sus límites

- Obtener las colecciones de escenas/fuentes que se quieren reconstruir, o dejar por escrito que Cortex 2.0 comenzará sin importarlas y que se crearán manualmente. El perfil inspeccionado no aporta escenas.
- Completar el despliegue de PCs con ubicación del navegador, OBS, Spotify, Rulo/medios, almacenamiento y servicios externos. El dibujo LAN/Tailscale es conceptual, no un inventario actual.
- Añadir contexto de seguridad y datos: quién puede usar cada control, qué datos entran/salen, retención, borrado, respaldos y recuperación. La autenticación por origen/navegador no debe asumirse como autorización suficiente para un panel remoto.
- Definir los límites de fallo: qué continúa si cae Spotify, OBS, Facebook, Cdelu/Firebase, IA o la base, y qué debe hacer el operador.
- Mantener la propuesta de proceso único como punto de partida, no requisito. Documentar justificación cuando un adaptador necesite proceso o host separado.

### C. Comparar lenguaje y empaquetado con un prototipo acotado

Cuando el propietario autorice la prueba, evaluar Go y C#/.NET bajo el mismo escenario no productivo. El prototipo debe incluir:

- aplicación Windows empaquetada y arranque/cierre observables;
- panel mínimo local y conexión OBS de ensayo, inicialmente de solo lectura;
- SQLite temporal/sintética con escritura concurrente, cierre inesperado, reinicio, respaldo y restauración;
- lista de dependencias nativas/transitivas, licencia, tamaño del paquete y requisitos del equipo;
- RAM/CPU en reposo y durante el flujo acordado, tiempo de arranque y reconexión, y claridad de los errores.

La prueba no necesita reconstruir módulos. Si Go gana en simplicidad de publicación y mantenimiento sin regresiones medibles, ratificarlo. Si Windows, el driver o el empaquetado añade fricción material, reconsiderar C#/.NET antes de escribir el producto.

## Riesgos que requieren atención temprana

- **Control remoto:** el objetivo de panel privado desde otras PCs debe definirse junto con ACL de Tailscale o política de LAN/firewall. No publicar el WebSocket de OBS ni controles administrativos en Internet. En la documentación/código del legado aparece un WebSocket de sorteos en el puerto 688; verificar autorización y exposición de red antes de decidir qué contrato debe conservar el reemplazo.
- **Relay UpCloud:** la transmisión depende de OBS, conectividad UDP saliente y un VPS remoto. El README describe bóveda DPAPI local y claves desplegadas en el servicio remoto; el reemplazo debe decidir cómo limitar, renovar y recuperar esas claves sin mostrarlas ni dejar la URL SRT en logs o capturas del panel.
- **Premios y operaciones externas:** la clave idempotente propuesta es adecuada, pero si Cdelu/Firebase recibe la operación y la respuesta se pierde, el estado queda ambiguo. Antes de reintentar se necesita consultar/reconciliar la operación con la misma identidad, o un mecanismo equivalente que el proveedor soporte.
- **Persistencia única:** centralizar SQLite puede ordenar el modelo, pero exige un dueño por tabla, transacciones claras, una estrategia de escritura concurrente y restauración probada. No asumir que compartir archivo equivale a compartir correctamente las reglas.
- **Captura de Facebook:** al depender de lo renderizado por una página cambiante, puede fallar sin aviso. El nuevo diseño necesita estado de conexión y un modo de recuperación/operación manual.
- **Recursos y efectos de prueba:** definir límites verificables entre simulación, ensayo con proveedor real y directo. No permitir que una simulación publique a OBS, cambie escena, otorgue premio o reproduzca música por accidente.

## Conclusión para el propietario

La organización por capacidades y un panel/backend compartidos puede simplificar Cortex. En la revisión IA 2, Go era el candidato preferido; después, el propietario eligió Go para el núcleo, como queda registrado en la adenda. La documentación aún no basta para empezar a reemplazar el sistema completo: faltan trazabilidad, escenas, seguridad y criterios de aceptación. Recomiendo un primer corte pequeño en Go que pruebe recuperación del servidor y operación unificada antes de reconstruir todos los módulos. Esa recomendación no autoriza implementación ni sustitución del Cortex actual.

## Evidencia técnica consultada

- Repositorio actual, `Control Cortex/backend/server.js`, ruta `/api/obs/trigger-hotkey` (alrededor de la línea 4244); lectura estática, sin ejecución.
- Repositorio actual, `Multi-Streaming upcloud/README.md`; lectura documental del flujo y operación descritos, sin inspeccionar la bóveda ni operar el VPS.
- [Go: abrir una base de datos y localizar/importar un driver](https://go.dev/doc/database/open-handle).
- [Referencia oficial de `database/sql`](https://pkg.go.dev/database/sql): el paquete no incluye drivers.
- [Driver `modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite): opción SQLite para Go sin CGO; su idoneidad para este producto no se ha probado.
- Documentos de IA 1: [inventario funcional](03-INVENTARIO-FUNCIONAL-Y-MATRIZ.md), [decisión tecnológica](04-REQUISITOS-Y-DECISION-TECNOLOGICA.md), [arquitectura](05-ARQUITECTURA-Y-PLAN-POR-FASES.md), [organización y diagramas](07-ORGANIZACION-PREVIA-Y-DIAGRAMAS.md), [estado y pendientes](06-ESTADO-Y-PENDIENTES.md).

## Adenda tras la descripción del flujo real

El propietario añadió como objetivo iniciar con pocas aplicaciones (idealmente Cortex y OBS), verificar comentarios de los vivos de dos cuentas Facebook, operar manualmente destacados/Rulo/bolilleros, cerrar con guardado confiable y recuperar fallas por acción automática o botón/comando. Las capturas aportan ejemplos visuales, pero no exportan escenas OBS.

Esta precisión **no cambia el dictamen condicional**, pero sí cambia los criterios de la comparación tecnológica: una prueba futura de Go contra C#/.NET debe incluir el inicio central, supervisión/reconexión y señal de captura individual por cuenta, además de OBS, SQLite sintética y empaquetado Windows. La captura actual usa una extensión de navegador, así que cualquier auxiliar que haga falta debe quedar bajo control de Cortex y no requerir que el operador lo abra o supervise por separado. El propietario aclaró que busca unificar la operación, no imponer que todas las dependencias sean un único proceso. No se probaron las cuentas ni se inició el servicio.

## Adenda: Trigger Studio y segunda PC

El propietario confirma que Cortex, la captura de Facebook, Spotify y OBS están hoy en la misma PC, y que la notebook futura será solo un control remoto. La documentación actual de Trigger Studio ya contempla un panel remoto; IA 2 recomienda integrarlo como sección del panel Cortex, sin backend ni agente local en la notebook. El panel/WebSocket actual de Trigger Studio no implementa autenticación propia, así que no se debe exponer tal cual; el acceso remoto nuevo debe pasar por controles de Cortex y una red privada autorizada.

## Adenda: decisión Go y recuperación del servidor

El propietario pregunta si crear Cortex 2.0 con Go y comunica su intención de elegirlo; IA 2 confirma que Go es una elección razonable para el núcleo/backend. El propietario informa también que el servidor Control Cortex actual a veces se cierra o requiere reinicio, y que lo recupera volviendo a ejecutar `Iniciar Control Cortex.bat`.

La elección de Go no elimina por sí sola ese tipo de fallo. Para cumplir el objetivo de recuperación, Cortex 2.0 debe tener supervisión de módulos y un mecanismo externo al proceso servidor que pueda detectar su cierre y reiniciarlo, además de registrar causa/estado y evitar bucles de reinicio. Esta decisión de lenguaje no autoriza por sí sola la reconstrucción completa.

