# Flujo operativo real y referencias visuales

**Fuente:** respuestas y capturas compartidas por el propietario el 12 de noviembre de 2026.  
**Estado:** describe el flujo reportado y el aspecto visual de ejemplos reales; todavía no es una especificación completa de escenas OBS.  
**Alcance:** reconstruir desde cero, sin migrar código. Todos los módulos salvo SocialStream Ninja continúan en alcance.

## Flujo confirmado por el propietario

### Antes del directo

- Quiere abrir la menor cantidad posible de cosas. Su objetivo ideal es iniciar el nuevo Cortex y OBS, sin abrir manualmente cada panel o módulo.
- La prioridad es unificar la operación: los módulos no deberían requerir lanzadores, ventanas ni supervisión manual por separado. Cortex debe administrar su ciclo de vida y mostrar el estado desde un panel central.
- En el método actual hay que abrir módulos uno por uno y algunos a veces se desconectan. El nuevo diseño debe eliminar esa carga operativa, detectar desconexiones y recuperarse sin que el operador tenga que descubrir qué módulo abrir de nuevo.
- El propietario informa que, en ocasiones, el servidor de Control Cortex se cierra o necesita reinicio. Hoy lo recupera ejecutando de nuevo el lanzador del sistema anterior.
- Antes de salir en vivo, necesita comprobar por separado que Cortex reconoce comentarios de los vivos de Facebook de **las dos cuentas**.
- La captura actual depende de una extensión de navegador. Si el nuevo diseño aún necesita un proceso auxiliar, Cortex debe iniciarlo, supervisarlo y recuperarlo; el operador no debería abrir ni cuidar ese módulo manualmente. No es requisito que todos los componentes sean literalmente un solo proceso de Windows.

### Durante el directo

- El historial muestra continuamente los comentarios nuevos.
- El operador elige manualmente un comentario del historial para destacarlo en el overlay.
- El operador controla manualmente las animaciones de Rulo; el detalle de si “formar a Rulo” significa elegir, preparar o entrenar una animación queda por precisar.
- El operador muestra bolilleros que ya compró para volver a lanzarlos. Mantener la activación bajo control manual; confirmar el flujo exacto y qué significa “volver a lanzar” antes de diseñar el control.
- Las capturas muestran distintas composiciones del programa con dos participantes en cámara, overlays de marca, música, Rulo, luces, comentarios destacados, ruleta y tablero de bolilleros. Son referencias del resultado visual, no exportaciones de escenas ni inventario exhaustivo de fuentes.

### Al terminar

- El operador cierra todo y actualmente confía en que la base guardó lo necesario.
- En Cortex nuevo, el cierre debe mostrar que no quedan escrituras pendientes y confirmar el guardado/cierre limpio. Si falla, mostrar qué dato quedó pendiente y cómo recuperarlo. La decisión sobre archivo histórico frente a base nueva sigue separada: no migrar código no decide por sí solo qué hacer con datos anteriores.

### Si algo falla

- El operador quiere pulsar un botón o comando para recuperarse, y espera que Cortex también intente solucionarlo solo.
- Diseñar dos niveles claros: recuperación automática de conexiones o módulos y controles manuales para reconectar, reiniciar un módulo y reiniciar Cortex completo.
- Mostrar qué se recuperó, qué sigue caído y cuándo hace falta una acción del operador. Una recuperación no debe duplicar premios, pedidos musicales, disparos de bolilleros ni acciones de OBS.
- El objetivo es reducir fallas evitables y que las desconexiones no obliguen al operador a adivinar qué cerrar/abrir; no es posible prometer que nunca habrá una caída. Cortex debe detectar, explicar, recuperar cuando sea seguro y ofrecer un camino manual claro.
- Si se cierra el proceso principal completo, la recuperación requiere un supervisor de Windows o un proceso supervisor separado que siga activo; un servidor no puede reiniciarse con código que ya dejó de ejecutarse. Esto debe quedar automático y transparente para el operador, con reintentos limitados y aviso si no logra volver.

## Requisitos observables que se desprenden

| Necesidad | Comportamiento que debe poder comprobarse |
|---|---|
| Operación unificada | Se inicia Cortex una vez y desde su panel se operan y supervisan todos sus módulos; OBS permanece como aplicación principal separada. Los procesos auxiliares necesarios quedan bajo control de Cortex. |
| Facebook, dos cuentas | Estado individual por cuenta: captura activa, vivo/cuenta reconocidos, hora del último comentario y aviso de desconexión. Un estado global “conectado” no basta. |
| Historial y destacado | Comentarios recientes de ambas cuentas en una vista continua; el operador selecciona cuál mostrar y puede comprobar qué se publicó en OBS. |
| Control de Rulo y bolilleros | Acciones deliberadas desde el panel, estado visible y confirmación del resultado; no activar durante pruebas aisladas ni por reinicio automático. |
| Cierre confiable | Indicador de escrituras pendientes, guardado durable y cierre ordenado; aviso si no pudo guardar. Respaldo/restauración debe tener un criterio acordado. |
| Recuperación | Reintentos acotados, estado de salud por componente y botones/comandos de reconexión o reinicio con resultado visible. Cortex evita instancias duplicadas, bucles de reinicio y repetición de efectos externos. |

## PCs y Trigger Studio

- El propietario confirma que actualmente Cortex, la captura de Facebook, Spotify y OBS corren en la misma PC.
- La segunda PC futura será una notebook usada solo como control remoto; OBS, Cortex y los módulos siguen en la PC principal.
- La notebook abre el panel central de Cortex por una red privada autorizada. Trigger Studio es una sección del panel, no otro programa que se instala y se abre en la notebook.
- No se necesita un agente ni backend en la notebook. Si en el futuro cambia el rol de esa PC, se revisa la arquitectura antes de agregar un proceso local.
- El panel HTTP/WebSocket actual de Trigger Studio carece de autenticación propia según su documentación. No reutilizarlo expuesto tal cual; el acceso remoto nuevo debe pasar por controles y autorizaciones del Cortex central.

## Referencias visuales aportadas

Estas imágenes permanecen en `img programa real`. Se enlazan como contexto visual; los nombres, comentarios y handles visibles en las capturas no se transcriben como datos del sistema.

- [Programa real — vista con participantes, música, marca y premio](<../img programa real/programa-real3.jpg>)
- [Programa real — tablero de bolilleros](<../img programa real/programa-real2.jpg>)
- [Programa real — ruleta](<../img programa real/ruletaprograma-real.jpg>)
- [Programa real — luces y animación](<../img programa real/Luces programa-real2.jpg>)
- [Programa real — comentario destacado y Rulo](<../img programa real/msj destacados programa-real.jpg>)
- [Programa real — respuesta de Rulo y overlay musical](<../img programa real/programa-real1.jpg>)

Las capturas ayudan a reconstruir qué ve la audiencia, pero no revelan nombres de escenas, jerarquía de fuentes, filtros, hotkeys, URLs Browser Source, pasos del operador ni configuración activa. No reemplazan la exportación de escenas o una guía escrita de esas relaciones.

## Decisiones que siguen abiertas

1. ¿Dónde corre cada vivo y cómo inicia sesión la captura de cada cuenta? Confirmar PCs, pestañas y recuperación de sesión, sin compartir contraseñas ni tokens.
2. ¿Qué botones exactos usa el operador para enseñar/seleccionar una animación de Rulo y para volver a lanzar los bolilleros comprados?
3. ¿Qué debe quedar guardado al cierre: comentarios, eventos, destacados, resultados de sorteos, estado de Rulo u otros datos? Indicar retención y qué información se puede descartar.
4. ¿Qué significa reiniciar “todo”: solo reconectar una integración, reiniciar el módulo responsable o reiniciar Cortex? Mantener visible el impacto antes de reiniciar.

## Consecuencia para lenguaje y arquitectura

El objetivo es tener un único punto de inicio y operación para los módulos de Cortex, con OBS aparte; no exige que cada dependencia sea literalmente parte del mismo proceso. La prueba de Go frente a C#/.NET debe comprobar inicio central, supervisión/reconexión de módulos, señal observable por cada cuenta de Facebook, OBS y SQLite sintética. La recomendación Go sigue provisional hasta esa prueba; el panel visual y la captura pueden requerir un proceso de navegador que Cortex debe administrar.

