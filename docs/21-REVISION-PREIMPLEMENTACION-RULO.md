# Revisión del ecosistema Rulo antes de implementar

**Fecha:** 14 de noviembre de 2026.  
**Estado:** revisión original con recomendaciones incorporadas al diseño; la implementación local actual cubre solo la base y el laboratorio iniciales.  
**Referencia vigente:** [Guía del ecosistema Rulo](20-GUIA-ECOSISTEMA-RULO.md).

## Dictamen

El diseño tiene sentido y no necesita otra arquitectura. Un único Cortex, eventos compartidos, Rulo como coordinador y habilidades con autoridad propia son una base coherente. También encajan el laboratorio con efectos simulados, el guardado explícito de reglas, la cola serial de Spotify, Cdelu como autoridad y la separación entre decisiones y presentación.

La documentación todavía no es una especificación cerrada de ejecución. Describe bien quién decide, pero deja abiertos varios pasos entre recibir un comentario y terminar su acción: recuperación, aclaraciones, resultados tardíos, borrado y restauración. Conviene cerrar esos contratos antes de programar sus componentes. No hace falta resolver voz, 3D ni todos los detalles de memoria para empezar la base y el laboratorio.

El propietario pidió incorporar estas recomendaciones después de recibir la revisión. Los hallazgos siguientes conservan su redacción original como antecedente; no son una segunda especificación. La guía 20 contiene los contratos actualizados y prevalece sobre las propuestas de este diagnóstico. Las funciones acordadas permanecen en alcance y las candidatas no se vuelven obligatorias por aparecer aquí.

## Incorporación de las recomendaciones

Los doce frentes quedaron tratados en la guía 20: respuesta lógica y resultados tardíos; sesión/vigencia y recuperación; operaciones/idempotencia/restauración; borrado; aclaraciones; tipos y aislamiento de entrada; laboratorio con prueba opcional; estados comunes; lista de cierre del lanzamiento; límites de memoria; guardado de reglas; y operación/carga. Se ajustaron el estándar 18, las referencias del documento 07 y AGENTS.md para remitir al mismo contrato.

El propietario resolvió las tres preferencias consultadas. Sobre operación: actualizar el historial y mostrar un aviso breve en overlay cuando llega un resultado tardío; al reanudar, recuperar solo pedidos recientes del mismo programa que aún no se enviaron, sujetos a revalidación. También confirmó incluir Cuaderno Vivo y las consultas de estadísticas del historial en el primer lanzamiento completo. Las tres preferencias consultadas quedaron resueltas; las menciones a candidatas o preguntas abiertas en los hallazgos originales de abajo son antecedentes, no el alcance vigente. Los valores de vigencia, retención, límites de carga y contratos remotos se concretan antes de habilitar las capacidades correspondientes; no se inventa una validación externa.

## Cobertura y evidencia

Se revisaron la guía 20 completa, AGENTS.md, README, el índice, el estándar 18 y la guía del historial 19; se contrastaron las referencias de arquitectura, inventario, pendientes, organización, operación y captura de los documentos 01, 03, 05, 06, 07, 10, 13 y 14. Se buscaron relaciones con Rulo en las guías de módulos existentes. Las descripciones del legado se toman de esas auditorías: esta revisión no constituye una nueva inspección integral del proyecto antiguo.

La lectura estática de [worker.go](../cmd/cortex-go/worker.go), [store.go](../cmd/cortex-go/store.go) y [facebook_overlay.go](../cmd/cortex-go/facebook_overlay.go) confirma estos puntos:

- El ingreso de comentarios no contiene un tipo general de evento ni identidad estable del participante. Después del guardado se notifica al overlay específico.
- Los eventos de prueba también se guardan en `events`, con origen `test:`. Compartir tabla no los convierte en entradas habilitadas para acciones reales.
- La deduplicación actual usa `source_id` y `dedup_key`. Borrar la fila también elimina esa protección de ingreso.
- El historial permite borrar físicamente eventos y guardar archivos independientes. Un cursor futuro no podrá recuperar el contenido de una fila borrada.

No se consultaron bases, credenciales ni datos de participantes. No se comprobaron contratos remotos actuales de Cdelu/Spotify, comportamiento en vivo, escenas OBS ni rendimiento. No se ejecutaron pruebas automatizadas ni compilaciones: el trabajo es documental.

## Correcciones y definiciones prioritarias

### 1. Una respuesta única frente a acciones que terminan después

**Problema:** la tabla «Propuesta concreta por frente» exige como máximo una respuesta por evento, incluso con plazo de respuesta. A la vez, Spotify debe confirmar reproducción y las habilidades pueden quedar pendientes o inciertas. Una canción en cola podría empezar bastante después de ese plazo.

**Propuesta:** distinguir una respuesta conversacional de las actualizaciones de su resultado. Mantener una respuesta lógica identificada por `responseId`, vinculada al `eventId`, con actualizaciones ordenadas. El historial conserva el resultado final; el overlay tiene una política explícita para actualizar o volver a mostrar un resultado tardío, sin repetir todo el mensaje ni ejecutar la habilidad otra vez.

Ejemplo: «Tu pedido quedó en cola; el 24 fue asignado» y, más tarde, una actualización de la parte musical cuando se verifique reproducción. «En cola» exige confirmación del dueño de esa cola y nunca equivale a «está sonando». La forma de mostrar actualizaciones tardías queda por elegir antes de conectar Spotify.

### 2. Recuperar eventos no significa ejecutar pedidos viejos

**Problema:** la guía exige ponerse al día desde el cursor, pero no define desde dónde empieza un consumidor nuevo, qué ocurre al reactivar Rulo o cuándo caduca un pedido. Recuperar todo podría ejecutar música de un programa anterior o intentar una reserva en otra lotería.

**Propuesta:** definir sesión de vivo, fuentes habilitadas y vigencia por clase de acción. Separar recuperar el estado de una operación ya enviada de iniciar una operación nueva. Los comentarios fuera de vigencia siguen consultables, pero no disparan efectos automáticamente. Una reserva debe quedar ligada a la lotería identificada para esa solicitud; un reintento no se redirige a la lotería que esté activa después.

El inicio de un consumidor debe fijar su punto inicial. Los eventos anteriores se exploran en el laboratorio; la recuperación normal continúa operaciones elegibles, con límites. Falta concretar cómo comienza/termina una sesión y qué hace la pausa con los pedidos aún no enviados.

### 3. Cursor, idempotencia y restauración necesitan un contrato conjunto

**Problema:** guardar un cursor y un ID local no basta para garantizar que nunca se repita una escritura externa. Puede caer Cortex después de que Cdelu acepte una reserva y antes de guardar su resultado. Restaurar un respaldo anterior también puede borrar el registro local de una acción que sí ocurrió fuera de Cortex.

**Propuesta:** registrar duraderamente la operación antes de enviarla y asociar una clave estable a cada intención materializada. La clave no se regenera al reintentar ni depende solamente de un índice de intenciones que otra interpretación del modelo podría cambiar. Separar cursor de entrada, estado de operación y confirmación externa; avanzar el cursor solo cuando el trabajo quede registrado de forma recuperable o descartado con motivo.

El despachador puede volver a entregar un evento: es el módulo dueño quien impide repetir su efecto. Tras una restauración, reconciliar las operaciones externas antes de reactivar escrituras. Verificar con Cdelu el soporte real de clave idempotente, consulta de resultado y duración de esas garantías. Si una integración no permite conocer el resultado, conservar `incierto` y ofrecer recuperación explícita; no prometer ejecución exactamente una vez solo con SQLite.

### 4. Borrado del historial frente a trabajo pendiente

**Problema:** el evento canónico es a la vez historial borrable y entrada recuperable. Además, la guía prohíbe copias innecesarias, mientras los archivos por programa ya son copias autorizadas. Hace falta distinguir ambos usos.

**Propuesta:** definir qué pasa con una operación no enviada, en curso y completada cuando se borra su comentario. El borrado no deshace una reserva o reproducción externa. Conservar, con retención explícita, solo el registro mínimo de operación necesario para evitar repeticiones y reconciliar; no conservar texto oculto indefinidamente.

No usar un borrado en cascada que elimine inadvertidamente toda evidencia de una acción pendiente. Las referencias de respuesta/evaluación deben tolerar «comentario eliminado». Los archivos y respaldos mantienen su borrado independiente, como ya establece el historial. El diseño debe contemplar también un comentario recapturado después de borrar su fila original.

### 5. Pedir aclaración requiere conservar el contexto de esa pregunta

**Problema:** está previsto preguntar por canción, artista o número, pero no cómo vincular «la de Soda» o «entonces el 25» con la solicitud anterior. `eventId` identifica un comentario; no identifica toda una conversación.

**Propuesta:** mantener aclaraciones breves con ID de interacción, evento original, campos faltantes, habilidad, sesión, participante verificable y vencimiento. La respuesta siguiente tiene su propio `eventId` y se enlaza a la interacción. Solo el módulo dueño decide cuándo la solicitud está completa.

Sin identidad suficiente no asociar dos personas por nombre: pedir reformular el pedido completo. No exigir identidad estable para toda charla o pedido musical autónomo; sí cuando haga falta vincular mensajes con seguridad y siempre para Cdelu. Definir qué ocurre si una persona tiene dos aclaraciones abiertas, cambia de pedido o responde después del vencimiento. Este contexto corto pertenece a Rulo base; no obliga a implementar memoria permanente de Cuaderno Vivo.

### 6. Evitar que respuestas y pruebas vuelvan a disparar Rulo

**Problema:** el historial común contendrá entradas, respuestas y resultados; hoy también contiene eventos sintéticos. Un consumidor que lea todo podría contestarse a sí mismo o ejecutar una prueba como si fuera un pedido real.

**Propuesta:** el contrato debe distinguir tipo, productor, ámbito vivo/laboratorio y correlación causal. El servidor establece esos metadatos; no se aceptan como permisos porque aparezcan en el texto del comentario o los sugiera la IA. Rulo procesa solo tipos y fuentes habilitados. Sus respuestas y los resultados de habilidades actualizan la interacción, no reinician el enrutamiento como comentarios de espectadores.

Persistir la respuesta antes de distribuir su señal visual, con identidad y orden para deduplicar reconexiones. El diagrama de la guía 20 debería mostrar explícitamente el registro de respuesta y resultado en el historial: actualmente solo dibuja la salida hacia presentación.

### 7. Laboratorio: prueba opcional y aislamiento comprobable

**Contradicción:** «Guardar regla» permite probar si se desea, pero las tablas de lecciones/propuesta y el paso 3 del ciclo redactan la prueba como obligatoria.

**Corrección propuesta:** «El administrador puede probar la propuesta antes de guardarla». Las validaciones de formato, dueño, permisos y conflictos conocidos se aplican al guardar; son distintas de ejecutar casos de evaluación. La aceptación previa al lanzamiento sí requiere revisar casos representativos.

El aislamiento debe imponerse en servidor mediante adaptadores simulados, no depender de una etiqueta de interfaz. Probar una regla candidata no la activa en vivo. Una prueba integrada solo puede usar un destino de pruebas realmente separado; si no existe, se mantiene la simulación. Mostrar el destino no vuelve inocua una llamada a producción.

### 8. Estados compartidos y varias intenciones

**Problema:** el estándar 18 enumera `aceptado`, `completado`, `rechazado`, `incierto`; la guía 20 añade `pendiente` y otras secciones omiten `aceptado`. No hay todavía una definición única de transiciones ni de la espera por aclaración.

**Propuesta:** definir una sola tabla de estados del ciclo de operación. Distinguir aceptación en cola, ejecución pendiente, finalización confirmada, rechazo y resultado incierto; definir también vencimiento/cancelación antes de envío. Una cancelación local no asegura cancelar un efecto externo ya enviado. La espera de aclaración pertenece a la interacción, antes de crear una operación ejecutable.

Congelar las intenciones una vez creadas las operaciones. Ejecutar las independientes sin que el fallo de una anule silenciosamente las otras. Las dependientes esperan su requisito; no compensar una reserva válida solo porque falló Spotify. Separar errores recuperables, rechazo de regla e incertidumbre para que no terminen en el mismo «falló».

### 9. Alcance del lanzamiento: cerrado en intención, abierto en enumeración

**Problema:** se promete lanzar todas las habilidades acordadas, pero la etapa 5 dice «por ejemplo, Cuaderno Vivo, consultas autorizadas, overlay y otras áreas de Cortex». La matriz llama a memoria «planificada» y a analítica «candidata». Eso permite interpretar cualquier idea nueva como condición del lanzamiento.

**Propuesta:** completar una lista finita con capacidad, estado de alcance, dueño, disparador, efecto, confirmación y criterio de cierre. Mantener todas las capacidades ya acordadas; separar lo confirmado que aún necesita diseño de lo candidato que todavía no está aprobado. Distinguir el alcance de todo Cortex del subconjunto de acciones que el público puede pedirle a Rulo.

Asignar el número elegido no es otorgar un premio, girar un bolillero ni cambiar una escena. Esas acciones conservan sus condiciones actuales. Voz externa y 3D siguen siendo futuras. «Integración transversal» puede significar solo lectura o ninguna capacidad pública; no obliga a automatizar cada módulo.

### 10. Memoria: precisar qué contexto puede salir del servidor

**Ambigüedad:** se propone que Cuaderno Vivo aporte contexto pertinente a Rulo, pero la etapa 5 dice que los datos de memoria y consultas personales no se envían a la IA. Tampoco se define si Rulo usa esos resultados solo con respuestas locales o puede recibir un resumen permitido.

**Propuesta:** conservar la restricción vigente hasta definir una política por campo y destino. Separar datos privados del servidor, hechos publicables y contexto permitido para redacción. No interpretar «memoria autorizada» como permiso general para enviarla a un proveedor o mostrarla en OBS.

La política debe indicar procedencia, acceso, corrección, vencimiento y borrado. El nombre/texto público puede seguir conteniendo datos personales aunque se quite el ID estable: seleccionar contexto mínimo, no prometer anonimización por eliminar un solo campo. La base conversacional puede avanzar sin recuerdos personales.

### 11. Guardar reglas no debe prometer detectar toda contradicción semántica

**Problema:** la guía exige detectar solapamientos y resolver especificidad entre instrucciones escritas en lenguaje natural, sin definir cómo. Convertirlo en una garantía absoluta haría innecesariamente complejo el laboratorio inicial.

**Propuesta:** dar estructura mínima a cada regla: dueño, categoría, condición/alcance, texto y estado. Detectar duplicados y conflictos estructurados comprobables; usar IA para señalar posibles contradicciones de redacción, con revisión del administrador. No presentarla como una prueba infalible de coherencia.

Guardar de forma atómica y detectar ediciones simultáneas desde dos paneles. Un contador de revisión para evitar pisar cambios no es un sistema de versiones/publicación. Definir en qué momento una interacción toma sus reglas y evitar que un cambio de personalidad reinterprete operaciones ya enviadas. Cada módulo vuelve a validar sus reglas de negocio actuales al ejecutar.

### 12. Falta concretar el control operativo del vivo

**Propuesta:** incorporar al diseño de Rulo controles de pausa general, silencio de conversación espontánea y habilitación por habilidad, con estados visibles. Distinguir silenciar la salida, impedir nuevas acciones y gestionar operaciones ya iniciadas. No asumir que apagar la animación detiene Spotify o cancela una reserva.

Definir colas acotadas, orden por interacción, prioridades, plazos y presupuesto de IA. Bajo sobrecarga, descartar primero oportunidades de charla espontánea; los pedidos explícitos deben terminar con un estado observable. El enrutamiento no debe bloquear la captura mientras espera la red. Los valores concretos pueden calibrarse en laboratorio, pero antes del vivo deben existir valores iniciales y límites máximos.

Mostrar al operador el motivo breve de la conducta —habilidad elegida, regla relevante, resultado o causa de silencio— sin almacenar razonamiento interno del modelo ni prompts completos. Conservar el control manual de animaciones y definir su prioridad sobre las señales automáticas al diseñar presentación.

## Qué simplificar o quitar de la documentación

No se propone quitar funciones acordadas. Sí reducir ambigüedad y duplicación:

- Mantener el orden de cinco etapas y los contratos en la guía 20; los otros documentos deben enlazarlos. El orden anterior de la sección de mejoras del documento 07 no debe competir con el vigente.
- Unificar «hallazgos», «propuesta por frente», «ciclo», «propuestas Spotify/laboratorio» y «decisiones que faltan»: hoy repiten reglas con pequeños cambios. Conservar decisiones confirmadas y pendientes en secciones distintas.
- Cambiar «pregunta en el chat» por «muestra la pregunta en overlay e historial; la persona responde desde Facebook». Así no reaparece implícitamente un emisor de comentarios que fue descartado.
- Reemplazar garantías absolutas de entrega única por entrega recuperable y efectos idempotentes o reconciliados. Documentar las limitaciones reales de cada servicio.
- No introducir ahora microservicios, otro bus/base, aprendizaje automático del transcript, un sistema de publicación de reglas ni 3D. Ninguno resuelve los huecos detectados.
- Enlazar la aceptación compartida de captura en lugar de repetir sus listas. Mantener separada la evidencia reportada por el propietario de las comprobaciones físicas que los documentos 13/19 aún marcan pendientes.

## Qué cerrar en cada etapa

| Momento | Definiciones necesarias | Comprobación manual o en laboratorio al implementar |
|---|---|---|
| Antes del contrato/despachador | Tipos, origen, ámbito, identidad opcional, cursor, vigencia, operación durable, borrado y restauración. | Reinicio entre guardado y despacho; evento repetido; evento sintético; comentario borrado y recapturado; restauración anterior a una acción. |
| Antes de la base/laboratorio | Contexto corto, aclaraciones, reglas candidatas/activas, estados y respuesta persistida. | Dos personas con el mismo nombre; respuesta a una aclaración vencida; salida propia que no vuelve al enrutador; prueba que no afecta el vivo. |
| Antes de Spotify | Confirmación de cola frente a reproducción, orden, resultados tardíos y recuperación incierta. | Tema aceptado que aún no suena; dispositivo caído; caída después de enviar; pedido duplicado; fallo parcial junto con otra habilidad. |
| Antes de Cdelu | Identidad estable verificable y enlazada, lotería objetivo, asignación atómica y contrato remoto de idempotencia/consulta. | Número ocupado; mismo número ya asignado; timeout con resultado desconocido; cierre/cambio de lotería; identidad ausente. Sin estas garantías no habilitar reservas. |
| Antes del lanzamiento completo | Lista finita de capacidades acordadas, pausa/carga, retención, salida pública y composición OBS. | Sobrecarga sin bloquear captura; pausa con operaciones en curso; proveedor IA caído; reconexión de overlay; control manual visual; escena real de OBS. |

No es una instrucción para ejecutar pruebas ahora ni para crear una suite automatizada. Es una lista de escenarios que el diseño debe poder explicar y que se comprobarán cuando exista la implementación correspondiente, respetando las instrucciones del repositorio.

## Recomendación de avance

Primero convertir los puntos 1–8 en contratos breves dentro de la guía 20 y completar el alcance de lanzamiento. Luego iniciar las etapas ya acordadas, cuando el propietario autorice implementar. Las decisiones específicas de memoria pueden cerrarse al entrar en esa habilidad; voz y 3D siguen fuera de la base inicial.

El agregado más útil ahora es cerrar el ciclo operativo de lo que ya se quiere construir. Más habilidades nuevas aumentarían el trabajo sin resolver cómo Rulo recuerda una aclaración, confirma un resultado tardío o se recupera sin repetir acciones.
