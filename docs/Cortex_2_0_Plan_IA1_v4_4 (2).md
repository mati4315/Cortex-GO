# Cortex — Bases y criterios para reconstrucción desde cero con Codex

**Versión de trabajo:** 4.4 — 12 de noviembre de 2026  
**Estado:** propuesta preliminar para análisis, **no aprobada**.  
**Propósito:** ofrecer bases para que una **primera IA realice una auditoría integral del ecosistema Cortex (código, OBS, Windows, integraciones, datos, operación real, rendimiento y necesidades futuras), proponga mejoras, elija y justifique el lenguaje principal y produzca documentación Markdown ordenada en una carpeta asignada por el propietario**; después, una segunda IA revisará y evaluará ese trabajo de forma independiente. **Codex será la tercera IA, encargada de implementar** solo tras autorización del propietario, con revisión posterior de la segunda IA.  
**Formato:** únicamente fundamentos, recomendaciones, comparaciones y criterios de revisión. **Sin código de programación, comandos, implementaciones ni instrucciones para modificar el repositorio.**

> **Vigencia:** este archivo conserva el encargo preliminar de IA 1 y sus instrucciones iniciales. El propietario después eligió Go, pidió continuar con la reconstrucción desde cero y autorizó el primer corte de implementación. Por lo tanto, las restricciones iniciales de “no programar” de este documento ya fueron superadas para el trabajo descrito en [`11-PRIMER-CORTE-PRUEBA-TECNICA.md`](11-PRIMER-CORTE-PRUEBA-TECNICA.md). Ese permiso no incluye reemplazar ni editar el Cortex anterior, integrar cuentas reales o modificar escenas de OBS.

> **Advertencia de contexto.** Este documento se elaboró sin inspeccionar el repositorio, las últimas actualizaciones, los módulos reales, los archivos de configuración de OBS, las sesiones reales de transmisión, las dependencias, las pruebas ni el desempeño del Cortex actual. Las áreas enumeradas son ejemplos y no deben tomarse como un inventario confirmado. Si alguna recomendación contradice una decisión documentada o una necesidad actual del proyecto, prevalecen las evidencias del proyecto y debe explicarse el motivo. **Codex no debe convertir estas hipótesis en requisitos confirmados.**

## INSTRUCCIONES DE INICIO — SI VOS SOS LA IA N.º 1, ESTE ES TU ENCARGO

**Rol asignado automáticamente por este documento: IA 1 — Investigadora, analista funcional y arquitecta de Cortex 2.0.** Si el propietario te entrega este archivo y te indica que sos la primera IA, **asumí ese rol directamente**. Este documento es tu guía de trabajo y no una aprobación para modificar el proyecto. No actúes como IA revisora ni como Codex implementador.

**Misión:** comprender el Cortex real —su código vigente, OBS, la PC Windows, los datos, las integraciones y el uso cotidiano—, detectar qué necesita realmente el propietario, proponer mejoras propias y **elegir con argumentos un lenguaje principal** para reconstruir Cortex desde cero. Prepará documentación clara que permita a otra IA cuestionar tu trabajo y luego a Codex ejecutar un plan aprobado.

**Tu primera acción, antes de hacer recomendaciones:**

1. Confirmar el acceso al **repositorio actualizado de Cortex** y a la **carpeta de documentación que el propietario te asignará**. Si la carpeta aún no fue indicada, pedí **solo ese dato esencial** antes de crear archivos. No inventes una ubicación.
2. Revisar este documento completo y crear un **índice de auditoría** en esa carpeta. Empezar por inventario y evidencias, no por escribir código ni por elegir Go de antemano.
3. Auditar el código y las dependencias; solicitar para examen **copias/exportaciones autorizadas de OBS** y, cuando proceda, datos del entorno Windows, bases de datos, logs y recursos. Nunca modificar la instalación de producción para investigar.
4. Contrastar los archivos con **cómo se utiliza Cortex realmente**. Preguntar al propietario por operaciones habituales, funciones ocasionales, funciones futuras y comportamientos que no pueden deducirse del código.
5. Generar **tus propios documentos `.md`** en la carpeta asignada y mantener actualizados el índice, la cobertura de auditoría, los riesgos y las preguntas pendientes.
6. Investigar problemas, oportunidades de simplificación y **mejoras originales** que no aparezcan en este documento. Separar ideas útiles de complejidad o funciones decorativas.
7. Definir requisitos funcionales **independientes del lenguaje** y una arquitectura mínima propuesta. Comparar Go, C#/.NET, Rust, PHP, la tecnología actual y C++ si hay requisitos nativos. **Elegir y justificar una recomendación concreta**, sin inventar pruebas.
8. Entregar un **paquete de traspaso** comprensible para IA 2: resumen ejecutivo, evidencia, propuestas, lenguaje elegido, alternativas descartadas, plan por fases para Codex, puntos débiles y decisiones que requieren autorización.

**Límites estrictos de IA 1:** trabajá en modo de análisis y documentación. **No edites el código fuente, no reconfigures OBS, no instales dependencias, no modifiques datos productivos, no comiences la reconstrucción y no apruebes tu propio plan.** Pedí autorización explícita si alguna medición requiere actuar sobre el entorno. La única escritura preautorizada es en los `.md` de la carpeta indicada por el propietario.

**Regla de claridad para todas las IA:** cada recomendación importante debe indicar **evidencia, grado de confianza, alternativa considerada, beneficio, riesgo y estado** (propuesta, pendiente, revisada o aprobada por el propietario). Si no pudiste comprobar algo, escribí **«no verificado»**; no completes las lagunas con suposiciones.

**Fin de tu misión:** cuando exista un conjunto de `.md` coherente y trazable que la IA 2 pueda revisar **sin depender del historial de chat**. Tu entrega no es un Cortex nuevo, sino **una auditoría y un plan de reconstrucción fundamentado**. La autorización de desarrollo la da únicamente el propietario.

### Equipo recomendado: solo tres roles, no cuatro equipos en paralelo

| Rol | Responsabilidad | Entrega o decisión |
|---|---|---|
| **IA 1 — Investigación y arquitectura (vos)** | Auditoría integral, requisitos, nuevas propuestas, elección justificada de lenguaje y planificación. | Documentación `.md` para revisión. **No implementa.** |
| **IA 2 — Revisión independiente** | Cuestiona hallazgos, verifica evidencias, detecta omisiones, corrige el plan y emite su dictamen. También revisa las entregas de Codex. | Aprobación técnica **recomendada** o correcciones. **No autoriza por cuenta propia.** |
| **IA 3 — Codex** | Reconstruye por etapas en un proyecto separado, hace pruebas y documenta resultados **después de la autorización del propietario**. | Implementación verificable y reportes para IA 2. |

**Propietario:** decide el alcance definitivo, responde las preguntas operativas y **autoriza** pasar de análisis a desarrollo y de pruebas a reemplazo. Si necesita una tercera opinión independiente, podrá solicitarla puntualmente; **no es un rol permanente obligatorio**.

---

## 1. Nuevo objetivo: reconstrucción deliberada, no migración automática

El objetivo actualizado es **evaluar una nueva versión de Cortex construida desde cero**, aprovechando que la versión existente está prácticamente terminada y permite conocer mejor el producto final. **La implementación estaría a cargo principalmente de Codex (IA de programación de OpenAI); el propietario no tiene previsto escribir personalmente el código.**

La intención no es trasladar todos los archivos del proyecto anterior ni reproducir sus decisiones técnicas por costumbre, sino **conservar las funcionalidades y comportamientos útiles mientras se simplifica la implementación**.

Principios:

- Utilizar Cortex actual como **referencia funcional verificable**: qué hace, cómo responde, qué datos conserva y de qué sistemas depende. **Cruzar esa evidencia con OBS real y con la operación habitual del usuario**; el código por sí solo no demuestra qué se usa.
- Hacer una **selección consciente** de lo que se reconstruye, lo que se elimina y lo que se rediseña.
- No perder comportamientos importantes por considerarlos detalles menores; documentar las excepciones, los casos límite y las necesidades operativas durante transmisiones.
- No confundir «desde cero» con «todo en un solo archivo», «todo en un solo lenguaje» o «ninguna dependencia externa».
- Mantener la versión actual utilizable mientras se desarrolla y valida la nueva; evitar reemplazarla antes de tener equivalencia comprobada.
- Considerar el rendimiento, pero también la **confiabilidad, facilidad de mantenimiento, integración, simplicidad y costo de evolución**.
- Buscar que el resultado sea **fácil de inspeccionar, probar y modificar mediante Codex**, sin depender de la memoria de una conversación concreta ni de un agente específico.
- Conservar como referencia versiones operativas, datos y criterios de aceptación: empezar desde cero no significa perder evidencia ni trazabilidad.

**Recomendación central:** rediseñar desde los requisitos observados, no desde las carpetas, tecnologías o errores históricos del proyecto anterior.

## 2. Hipótesis arquitectónica principal: un programa con módulos internos

Evaluar un **monolito modular** como punto de partida:

- Un **Cortex principal para Windows** responsable del inicio, configuración global, supervisión, diagnósticos y cierre ordenado.
- **Componentes funcionales internos** con responsabilidades delimitadas y dependencias explícitas.
- Un mecanismo simple y predecible para comunicar módulos mediante operaciones directas y eventos internos, según corresponda.
- Servicios compartidos solo cuando realmente reduzcan duplicación: configuración, persistencia, logs, permisos y acceso a recursos.
- Capacidad de **activar, desactivar o reiniciar funcionalidades cuando sea seguro y útil**; estas capacidades requieren definir claramente qué ocurre con las tareas y el estado pendiente.

**Distinción importante:** un único programa principal **no exige un único proceso del sistema operativo en todos los casos**. OCR, captura, acceso a dispositivos, bibliotecas nativas o ciertos entornos gráficos podrían requerir procesos auxiliares. La recomendación es supervisarlos desde Cortex y evitar procesos adicionales sin motivo.

Un fallo grave dentro del proceso principal podría afectar a todos los módulos. Evaluar aislamiento específico solo en las funciones cuyo riesgo o costo de fallo lo justifique.

## 3. Elección del lenguaje: comparar antes de decidir

**Go es el candidato inicial**, pero no debe presentarse como lenguaje aprobado solo por su reputación de eficiencia. C#/.NET y Rust son alternativas importantes; C++ puede ser adecuado para extensiones nativas de OBS. **PHP también debe considerarse sin prejuicios**, ya que el propietario lo conoce y le agrada, aunque su conveniencia como núcleo Windows debe demostrarse. La tecnología vigente de Cortex debe conservarse como línea de comparación.

**Cambio de criterio por el uso de Codex:** la preferencia o familiaridad del propietario con PHP no tiene que decidir por sí sola el lenguaje, porque Codex realizará la mayor parte de la implementación. Sin embargo, la comprensión del propietario, su capacidad de revisar resultados y su comodidad para operar el proyecto **siguen siendo valiosas**. Codex puede desarrollar en varios lenguajes; eso no elimina los riesgos de integración, depuración o despliegue.

| Alternativa | Motivos para evaluarla | Posibles compromisos |
|---|---|---|
| **Go** | Ejecutable nativo para Windows; concurrencia; herramientas integradas; diseño simple para servicios, eventos y conexiones permanentes. | Integraciones gráficas o nativas de Windows pueden requerir bibliotecas, puentes o procesos adicionales; el recolector de basura no implica consumo mínimo garantizado. |
| **C# / .NET** | Integración sólida con Windows, herramientas de diagnóstico y desarrollo, ecosistema de escritorio; opciones de distribución independiente y compilación nativa. | El consumo, el empaquetado y las restricciones dependen del modelo de publicación y de las bibliotecas; Native AOT no siempre es compatible con todo el ecosistema. |
| **Rust** | Control preciso de recursos, seguridad de memoria sin recolector de basura y rendimiento muy alto; útil para partes intensivas o sensibles a latencia. | Mayor curva de aprendizaje y complejidad de desarrollo; no siempre reduce el costo global del proyecto. |
| **C++** | Acceso nativo a OBS y a APIs de bajo nivel; adecuado para plugins integrados al proceso de OBS. | Más exigente en mantenimiento y seguridad de memoria para un núcleo general si no hay necesidad concreta. |
| **PHP** | Gran capacidad para APIs, paneles web y SQLite; CLI y bibliotecas asíncronas permiten procesos persistentes; afinidad del propietario con el lenguaje. | Empaquetar un único ejecutable Windows no es su vía habitual; la gestión de procesos largos, concurrencia y extensiones puede complicar el núcleo frente a Go o C#. |
| **Tecnología actual / Node.js si aplica** | Reutilización de conocimiento, bibliotecas e integraciones ya probadas; velocidad de desarrollo. | Debe medirse su costo real; no suponer que el runtime sea el principal cuello de botella. |

### Criterios de comparación obligatorios

1. **Consumo medido:** RAM y CPU en reposo, con OBS activo, durante transmisiones y en picos de eventos.
2. **Latencia y estabilidad:** tiempo de respuesta, colas, reconexión y comportamiento ante errores.
3. **Compatibilidad:** OBS, APIs y fuentes externas, SQLite, captura, OCR, dispositivos, interfaz, servicios de Windows y bibliotecas necesarias.
4. **Distribución:** tamaño, instalación, actualizaciones, dependencias, detecciones de antivirus, permisos y recuperación.
5. **Facilidad de evolución:** depuración, pruebas, documentación, velocidad para añadir y mantener funcionalidades.
6. **Complejidad real:** cantidad de procesos auxiliares, adaptadores y excepciones que introduce cada alternativa.
7. **Trabajo con Codex:** claridad de los errores y pruebas, capacidad de generar cambios pequeños y revisables, calidad de herramientas de compilación y validación, facilidad para reproducir fallos y mantener el proyecto entre sesiones.
8. **Control del usuario:** que decisiones importantes sean transparentes y auditables sin exigir conocimientos profundos del lenguaje elegido.

No dar porcentajes ni prometer ahorros de RAM/CPU sin pruebas comparables. Una mejora local de lenguaje puede no modificar el rendimiento global si el mayor costo está en OBS, OCR, navegador, captura o dispositivo externo.

**Hipótesis inicial de este documento (no decisión final):** Go merece ser la primera alternativa a prototipar; **C#/.NET es su competidor más importante en un producto Windows**. Rust debe probarse si existen necesidades concretas de uso extremo de recursos o latencia. PHP debe evaluarse honestamente, especialmente para interfaces o servicios, sin forzar su adopción ni su exclusión. C++ debe reservarse principalmente para componentes nativos que lo justifiquen. Aceptar una solución híbrida solo si aporta una ventaja clara y verificable; **no sumar PHP, Go y otros runtimes por costumbre**, si el objetivo es simplificar.

## 4. Relación con OBS: importante distinguir dos modelos

**OBS Studio está desarrollado principalmente en C y C++; sus plugins nativos suelen escribirse en C++ y compilarse como bibliotecas dinámicas.** No se necesita utilizar C++ para todo Cortex por ese motivo.

Distinguir:

- **Cortex como programa externo:** controla OBS desde fuera mediante interfaces oficiales como **OBS WebSocket**, cuando cubran los requisitos. Go, C# y otros lenguajes pueden utilizar este enfoque.
- **Extensión que se ejecuta dentro de OBS:** si alguna función exige acceso directo al motor o a capacidades que WebSocket no expone, estudiar un plugin nativo de OBS, normalmente en C++.
- **Scripts y recursos de OBS:** algunos comportamientos pueden resolverse con funciones, scripts o fuentes ya existentes sin ampliar el núcleo de Cortex.

**Regla recomendada:** privilegiar la integración externa cuando alcance. Introducir plugins nativos únicamente por una limitación comprobada o una necesidad de rendimiento real, valorando el riesgo de compatibilidad entre versiones de OBS.

Antes de diseñar cada componente, identificar si necesita **controlar OBS**, **recibir eventos**, **mostrar contenido** o **alterar capacidades internas de OBS**. No son equivalentes.

### Auditoría de OBS como fuente independiente de requisitos

**La primera IA no podrá saber qué usa el propietario en OBS examinando solamente el Cortex anterior.** Antes de recomendar qué reconstruir o eliminar, deberá contrastar tres fuentes: **(1) el código de Cortex, (2) la configuración real de OBS y sus dependencias y (3) el uso observado durante transmisiones o simulaciones representativas**.

Con acceso autorizado por el propietario, examinar **copias de solo lectura**, nunca el entorno operativo original, de las colecciones de escenas, perfiles y, cuando corresponda, la configuración de plugins y scripts. En OBS Studio se pueden exportar colecciones de escenas mediante **Colección de escenas > Exportar** y perfiles mediante **Perfil > Exportar**. En una instalación habitual de Windows, la configuración suele encontrarse en la carpeta de usuario **`%APPDATA%\obs-studio`** (y puede estar en otro lugar si OBS se ejecuta en modo portátil). Las colecciones de escenas describen escenas y fuentes; los perfiles guardan principalmente configuración de salida. No son sustitutos entre sí.

Verificar, cuando aplique: escenas y fuentes; estados visibles e invisibles; filtros y transiciones; fuentes de navegador, archivos, rutas y direcciones locales; hotkeys y automatizaciones; plugins y scripts; audio, video y perfiles; conexiones OBS WebSocket; assets y aplicaciones que suministren contenido. Una exportación de OBS **no incluye necesariamente los archivos físicos externos** ni todas las configuraciones de los plugins, por lo que se deben identificar dependencias adicionales sin asumir que el archivo exportado está completo.

**No confundir “configurado” con “utilizado”.** Una fuente invisible, una escena que casi nunca se abre, una función sin actividad reciente o un plugin no mencionado por Cortex pueden ser esenciales en casos puntuales. La primera IA deberá documentar con evidencias de funcionamiento normal y excepcional qué se activa, qué acciones lo disparan y qué resultado produce. Si el propietario no permite observación directa, registrar las preguntas o comprobaciones pendientes; no declarar funciones «en desuso» solo por inspección estática.

**Seguridad y límites:** antes de compartir exportaciones o logs, crear respaldo y revisar información sensible: claves de transmisión, contraseñas de WebSocket, tokens, URLs privadas, identificadores, nombres personales y rutas locales. No copiar secretos a los Markdown. No modificar OBS, su configuración, escenas, fuentes, plugins ni la instalación actual durante la auditoría sin autorización expresa.

## 5. Organización conceptual interna

Las siguientes áreas son **hipótesis**, no una lista certificada del Cortex actual:

| Área | Posible responsabilidad |
|---|---|
| **Núcleo** | Ciclo de vida, estado general, inicialización, supervisión, cierre y coordinación básica. |
| **Integración con OBS** | Conexión, reconexión, eventos de OBS, operaciones permitidas y conocimiento del estado. |
| **Entradas / captura** | Recepción o adquisición de señales desde las fuentes realmente utilizadas. |
| **Normalización de eventos** | Transformar señales de entrada en eventos internos claros y verificables. |
| **Participantes** | Identificación, registros e historiales, solo si forman parte del alcance actualizado. |
| **Reglas y automatizaciones** | Evaluar condiciones y activar acciones evitando acoplarse a un proveedor específico. |
| **Datos** | Persistencia coherente, migraciones de esquema, respaldo y recuperación. |
| **Medios y recursos** | Referencias a imágenes, videos, audios y materiales empleados por las escenas. |
| **Interfaz de gestión** | Configuración, estado, diagnósticos y control de funciones. |

No separar cada botón, evento o función pequeña como un módulo. Delimitar por **responsabilidad, evolución independiente, reglas propias y dependencias reales**. Que Codex genere el código no justifica añadir capas, patrones, frameworks o abstracciones «por si acaso».

### Términos en Go, si se elige ese lenguaje

- **Módulo Go:** unidad de código y dependencias, normalmente administrada como un proyecto.
- **Paquete Go:** agrupación de código relacionado.
- **Módulo funcional de Cortex:** concepto de producto y arquitectura; puede corresponder a uno o varios paquetes.

**Recomendación:** empezar con **un módulo Go principal y paquetes internos** si no existe necesidad de versionar o distribuir bibliotecas por separado. No confundir módulos funcionales con plugins dinámicos. El mecanismo nativo de plugins de Go no debe darse por disponible para Windows.

## 6. Comunicación interna: simple antes que sofisticada

Evaluar dos formas complementarias:

- **Operaciones explícitas** cuando un componente debe solicitar una acción y conocer su resultado.
- **Eventos internos** cuando una fuente anuncia un hecho y varios consumidores independientes podrían reaccionar.

Recomendaciones:

- Documentar nombres, significado, datos mínimos y responsables de cada evento.
- Diferenciar **hechos ocurridos** de **estados actuales**.
- Definir qué ocurre con eventos duplicados, tardíos, fallidos o recibidos durante la desconexión.
- Establecer límites para las colas, prioridades y políticas frente a sobrecarga.
- Persistir únicamente los eventos que deban sobrevivir a un cierre o reinicio.
- Evitar sistemas de mensajería externos o capas distribuidas si la comunicación dentro del programa resuelve el problema.

**Ejemplo a reconfirmar:** si el proyecto sigue registrando personas que se unen a Facebook, una misma persona debería poder producir entradas distintas en momentos diferentes. La fotografía de perfil sería opcional y la captura de nombre, texto y marca temporal debería contrastarse con la implementación actual. No imponer deduplicación por identidad sin validar el caso de uso.

## 7. Base de datos y continuidad de información

Si sigue siendo adecuada, **SQLite** es una opción razonable para datos locales. No se recomienda sustituirla solo por reescribir el programa.

- Definir responsables de los datos por área funcional.
- Evitar acceso descoordinado de múltiples componentes a operaciones de escritura.
- Considerar transacciones, bloqueos, concurrencia y recuperación ante interrupciones.
- Versionar cambios de estructura de datos.
- Preservar historiales, identificadores, configuraciones y vínculos que tengan valor.
- Verificar copias de seguridad **y restauración** antes de usar datos reales en la versión nueva.
- Definir límites de retención y protección de credenciales o información personal.

La versión nueva no tiene obligación de conservar el **diseño interno** de la base anterior, pero sí los **datos y comportamientos** que deban mantenerse. Preparar una ruta de conversión o importación solo cuando corresponda.

## 8. Herramientas auxiliares y procesos externos

No imponer «cero dependencias» como requisito absoluto. Un programa principal puede administrar herramientas que sean difíciles o ineficientes de reemplazar.

Si siguen vigentes mecanismos como captura desde dispositivos, scrcpy, ADB, OCR o motores multimedia, confirmar su presencia y evaluar:

- Necesidad real de cada herramienta.
- Costo medido en RAM, CPU, disco y tiempo de arranque.
- Licencia, compatibilidad con Windows, instalación y mantenimiento.
- Arranque, supervisión, cierre y recuperación desde Cortex.
- Degradación controlada si una herramienta deja de responder.

**No reimplementar un motor de OCR o captura por principio.** Comparar el costo de usar uno existente frente al desarrollo y mantenimiento de una alternativa propia.

## 9. Interfaz y distribución para Windows

Evaluar primero lo que la versión actual necesita y lo que sus usuarios realmente utilizan.

Opciones conceptuales:

- **Interfaz web local:** puede aprovechar Vue/TypeScript si resulta conveniente y evita rehacer componentes valiosos.
- **Interfaz nativa de Windows:** considerar C#/.NET si ofrece integración y mantenimiento claramente superiores.
- **Interfaz alojada por un contenedor de escritorio:** evaluar el motor requerido y su efecto en tamaño, memoria, instalación y soporte.

Que el programa tenga un único ejecutable principal no significa automáticamente que **incluya un navegador, OCR, drivers o todo runtime del sistema**. Documentar cada dependencia y decidir si se instala, se empaqueta o se utiliza la existente.

El panel debería centrarse en estado, errores, conexiones, funciones activas y configuración comprensible. No crear un panel nuevo por razones estéticas si el actual ya cumple los requisitos.

### Control remoto privado (requisito deseado, sujeto a validación)

El propietario desea poder **abrir Cortex en la PC principal con OBS** y realizar operaciones como **destacar comentarios y ejecutar acciones sobre OBS desde otra computadora** conectada a la **misma red local**; los equipos también disponen de **Tailscale**. No desea crear usuarios ni pantallas de inicio de sesión adicionales para este escenario.

**Hipótesis de mínima complejidad:** un solo Cortex ejecuta el núcleo y mantiene los datos en la PC principal; las demás computadoras abren **un panel web local/remoto**, sin iniciar otras instancias completas de Cortex. Esta solución es preferible inicialmente a distribuir el mismo `.exe` como segundo controlador, salvo que el análisis demuestre una ventaja concreta.

**Condiciones:** acceso limitado a los dispositivos/redes de confianza (por LAN o Tailscale y controles de acceso de red), sin exposición pública de controles administrativos ni de OBS WebSocket. No asumir que «sin login de Cortex» significa «sin medidas de seguridad». La IA 1 debe verificar que esta capacidad no esté ya resuelta y documentar su prioridad, coste, acciones autorizadas y requisitos de red antes de incluirla como funcionalidad aprobada.


## 10. Diseño mínimo, seguridad y calidad

Adoptar como criterio **«la mínima complejidad que cubra todos los requisitos»**, no «la mínima cantidad de código a cualquier precio».

Buenas prácticas para evaluar:

- Tareas en segundo plano cancelables y cierre ordenado.
- Reintentos controlados y reconexión sin bucles infinitos.
- Diagnósticos útiles sin registrar credenciales ni datos sensibles innecesarios.
- Límites de memoria y colas donde el volumen lo requiera.
- Acceso local seguro; autenticación cuando una interfaz esté expuesta a otras máquinas.
- Pruebas durante transmisiones reales o simuladas con OBS.
- Mecanismos de actualización y reversión conocidos.
- Fallos de un componente gestionados sin pérdida de datos, en la medida técnicamente posible.

Evitar introducir desde el inicio: microservicios, contenedores obligatorios, múltiples bases de datos, un sistema genérico de plugins descargables, un bus distribuido o arquitecturas extensibles complejas sin requerimientos comprobados.

**Criterio para aceptar cada dependencia o módulo:** debe resolver una necesidad identificada, aportar un beneficio mensurable o reducir un riesgo real. Evitar reemplazar componentes estables solo para obtener una arquitectura aparentemente más elegante.

## 11. Estrategia de reconstrucción desde cero

La reconstrucción no debería empezar por elegir nombres de carpetas o convertir archivos entre lenguajes. Secuencia recomendada:

**Etapa A — Auditoría integral de Cortex, OBS y operación real por la primera IA**

- Inventariar todas las funciones y flujos actualmente utilizados.
- Solicitar autorización y acceso a **exportaciones o copias de solo lectura de la configuración de OBS**; analizar escenas, fuentes, perfiles, plugins, scripts y dependencias pertinentes. Si no están disponibles, marcar el alcance incompleto.
- Contrastar código y configuración con el **uso real**, por observación autorizada o descripción verificable de transmisiones normales y casos ocasionales; no asumir que una escena inactiva es prescindible.
- Identificar cuáles son indispensables, opcionales, obsoletos, duplicados o experimentales, **sin deducir «prescindible» de «no se observó su uso»**.
- Completar una **auditoría integral del ecosistema**, además de OBS: entorno Windows; integraciones externas; bases de datos y archivos; recursos multimedia; flujos reales; RAM, CPU y latencia; errores y recuperación; necesidades futuras expresadas por el propietario.
- Documentar qué depende de OBS, de Windows, de Internet, de dispositivos y de servicios externos.
- Observar o reconstruir con evidencia tres momentos: **antes, durante y después de una transmisión**, incluyendo funciones ocasionales y recuperación de fallos.
- Registrar configuraciones, formatos de datos, comportamientos y casos límite.
- Identificar los problemas actuales demostrables y lo que **no** debe cambiar.
- Mantener un **registro de ausencias de evidencia y preguntas para el propietario**; no dar por completa una auditoría con accesos pendientes.

**Etapa B — Propuesta técnica y documentación de la primera IA**

- Crear una especificación funcional mínima pero completa, tomando como base el inventario. Para cada función relevante, incluir comportamiento esperado, entradas, salidas, errores habituales y cómo verificarla.
- Establecer límites de módulos, esquema de comunicación y reglas de persistencia.
- Escoger una propuesta de interfaz y distribución.
- Registrar explícitamente las funciones que no se reconstruirán y por qué.

**Etapa C — Evaluación y validación de la elección tecnológica**

- Probar la tecnología candidata con una integración real y acotada, preferiblemente una ruta de eventos significativa hacia OBS.
- Comparar Go con C#/.NET y, si hay justificación, Rust o PHP; no necesariamente prototipar el producto entero en cada alternativa. La prueba debe incluir funcionamiento desde Codex, empaquetado Windows y la integración real, no solo un microbenchmark.
- Medir desempeño, mantenimiento, compatibilidad y facilidad de distribución con el mismo escenario.
- La **primera IA debe escoger explícitamente un lenguaje principal y explicar sus motivos, ventajas, inconvenientes y alternativas descartadas**. Si no puede medir o ejecutar pruebas, dejar la decisión técnica propuesta como condicionada a validaciones concretas; nunca inventar resultados.

**Etapa D — Construcción controlada**

- Reconstruir por capacidades verificables, manteniendo la versión existente operativa. Codex debe trabajar por cambios acotados, revisables y con pruebas, sin alterar la instalación o los datos de producción por iniciativa propia.
- Validar funcionalidad, tiempos, estabilidad y datos después de cada área terminada.
- Mantener trazabilidad entre funciones antiguas y nuevas, incluidas las decisiones de eliminación.

**Etapa E — Sustitución segura**

- Realizar pruebas completas en una configuración de OBS representativa.
- Respaldar y verificar datos antes del cambio.
- Planificar instalación, reversión y coexistencia limitada cuando sea necesaria.
- Retirar la versión anterior **solo después** de demostrar que la nueva cumple los requisitos aprobados y el propietario haya autorizado explícitamente el reemplazo.

Esto constituye una **reconstrucción limpia con transición controlada**, no una traducción automática de código ni una orden de migrar inmediatamente.

## 12. Proceso de trabajo: IA 1 → IA 2 → autorización → Codex (IA 3)

**Tres roles de IA en total:** la primera investiga y propone, la segunda revisa y verifica, y la tercera es **OpenAI Codex**, que implementará únicamente después de recibir autorización. No hace falta una cuarta IA fija para aprobación final; **la aprobación del proyecto corresponde al propietario**.

**Secuencia y puntos de control:**

1. **IA 1 — Auditoría y propuesta:** analiza código y entorno actualizado de Cortex, OBS, Windows, integraciones, datos, recursos, rendimiento y uso real; propone mejoras propias, selecciona y justifica un lenguaje principal, diseña arquitectura y genera `.md` en la carpeta asignada.
2. **IA 2 — Revisión independiente:** examina también las fuentes cuando sea posible, detecta errores y omisiones, cuestiona el lenguaje elegido, identifica pruebas faltantes y emite un informe de aceptación, aceptación condicionada o correcciones. **No se limita a estar de acuerdo con IA 1.**
3. **Propietario — Decisión:** revisa las propuestas y el dictamen de IA 2, responde a preguntas críticas y **autoriza de forma explícita** si Codex puede iniciar la reconstrucción.
4. **IA 3 — Codex:** implementa **por capacidades completas y comprobables** en un proyecto separado, documenta decisiones y pruebas y conserva Cortex actual sin alterarlo. Cada etapa debe tener criterios de aceptación y una forma de detectar regresiones.
5. **IA 2 — Revisiones periódicas:** comprueba hitos de Codex contra requisitos, pruebas y documentación; exige correcciones cuando corresponda. **No se incorpora otra IA distinta para cada hito.**
6. **Propietario — Puesta en producción:** valida el funcionamiento real con OBS y autoriza expresamente reemplazar la versión anterior cuando exista evidencia suficiente y un plan de reversión.

**Documentación compartida y fuente de verdad:** `00-INDICE.md` y un documento de estado/decisiones aprobadas deben indicar siempre qué es vigente, qué está pendiente, quién propuso cada cambio, qué verificó IA 2 y qué aprobó el propietario. Evitar que agentes diferentes interpreten versiones antiguas como la decisión actual.

**Regla de separación:** IA 1 e IA 2 pueden analizar y documentar dentro de permisos autorizados; **no deben cambiar el código ni la instalación operativa**. Codex solo implementa cuando el propietario aprueba. Si una sola IA hace dos roles, se debe declarar: **una autorrevisión no reemplaza la verificación independiente**.

## 13. Instrucciones principales para la primera IA: auditoría completa y elección de lenguaje

Esta sección es el **encargo prioritario** a la primera IA. Tendrá acceso al Cortex actualizado y deberá basarse en hechos del proyecto, no en hipótesis de este documento.

### 13.1. Inspeccionar el proyecto completo antes de recomendar una reconstrucción

- Analizar el **código fuente completo**, estructura, configuraciones pertinentes, documentación y cambios recientes. No limitarse a los archivos principales ni a las funciones mencionadas aquí.
- Identificar qué hace cada componente, cómo se relacionan entre sí, cuáles son las dependencias reales, qué procesos o servicios utiliza y cómo se comunica con OBS y Windows.
- Reconstruir los flujos completos de trabajo: desde entradas, eventos y automatizaciones hasta acciones, persistencia, interfaz y salida hacia OBS.
- Examinar módulos existentes, funciones compartidas, utilidades, bibliotecas, integraciones, manejo de errores y recuperación; **verificar qué está implementado, qué está incompleto y qué ya no se utiliza**.
- Identificar versiones, restricciones, datos persistentes, credenciales o secretos que requieren protección y decisiones anteriores que no deben perderse.
- Distinguir lo **confirmado en el código**, lo **documentado pero no comprobado** y lo **supuesto**. Registrar rutas o referencias a la evidencia sin copiar claves ni datos sensibles.
- No afirmar que la auditoría es total si hay carpetas, accesos, archivos, herramientas o entornos que no se pudieron examinar. Indicar exactamente lo pendiente.

### 13.1.b. Correlacionar Cortex con OBS y documentar el uso real

Además de inspeccionar el código, **la primera IA debe auditar OBS como sistema complementario**. El propietario proporcionará acceso o exportaciones de forma explícita; si no las proporciona, la IA debe identificar el faltante y solicitarlo sin inventar resultados.

- Identificar **qué colecciones de escenas y perfiles se usan actualmente**, cuáles son de pruebas, respaldo o eventos especiales, y qué fuentes o filtros intervienen en los flujos relevantes.
- Catalogar fuentes de navegador, imágenes, video, audio, escenas anidadas, transiciones, scripts y plugins vinculados a Cortex o a operaciones importantes del directo; registrar rutas y dependencias sin exponer datos privados.
- Analizar **cómo se comunican Cortex y OBS**: solicitudes y eventos WebSocket, cambios de escena, cambios de visibilidad, contenidos mostrados y automatizaciones o acciones manuales. No asumir que toda interacción ocurre por WebSocket.
- Determinar qué mecanismos **están configurados**, cuáles **se observan en uso**, cuáles **se usan ocasionalmente** y cuáles **no se han podido confirmar**. Para recomendar eliminar algo, justificarlo con pruebas o una confirmación del propietario.
- Considerar los archivos auxiliares **fuera de la carpeta de configuración de OBS**: multimedia, overlays locales, sitios web, bases de datos, plugins o programas externos; la exportación de escenas por sí sola puede no incluirlos.
- Registrar las diferencias entre «lo que implementa Cortex», «lo que OBS tiene configurado» y «lo que el usuario efectivamente hace». Resolver contradicciones con evidencia o dejarlas como pendientes.
- Priorizar una **matriz de trazabilidad Cortex ↔ OBS ↔ uso real** que indique para cada función: origen, disparador, acción sobre OBS, recurso dependiente, frecuencia o contexto de uso, evidencia, criticidad y recomendación preliminar.
- Preservar **casos excepcionales** (eventos especiales, sorteos, escenas alternativas, tareas manuales, recuperación de fallas) aunque no aparezcan durante una única transmisión de prueba.
- Realizar esta investigación **sin modificar la instalación operativa de OBS ni recopilar credenciales en los archivos `.md`**. Si hace falta observación en vivo, coordinarla con el propietario y no interferir en emisiones reales.

**Regla de decisión:** ninguna función se elimina de la propuesta de Cortex 2.0 únicamente porque no aparece en el repositorio, no está visible en OBS o no se activó durante una sesión. La primera IA debe etiquetarla como **confirmada, ocasional, no confirmada o propuesta para retiro** y explicar el fundamento.

### 13.1.c. Auditoría integral obligatoria del ecosistema Cortex

**Esta fase es obligatoria antes de dar por terminada la propuesta de arquitectura o de recomendar definitivamente un lenguaje.** La primera IA debe investigar todas las áreas accesibles, indicar qué evidencia encontró y distinguir lo que no pudo comprobar. No necesita inventar módulos ni asumir que todos los componentes existen en la versión actual.

| Área que debe revisar | Preguntas que debe resolver |
|---|---|
| **Código y dependencias de Cortex** | ¿Qué funcionalidades y procesos existen, cuáles están terminados y cuáles dependen de componentes externos? ¿Hay duplicaciones o responsabilidades mezcladas? |
| **OBS real** | ¿Qué escenas, perfiles, fuentes, filtros, scripts, plugins, recursos y acciones de control están configurados? ¿Cuáles tienen evidencia de uso habitual u ocasional? |
| **Entorno Windows** | ¿Qué servicios, tareas programadas, procesos, controladores, permisos, dispositivos, instalaciones y rutas requiere el sistema para funcionar? |
| **Integraciones externas** | ¿Cómo se relaciona Cortex con APIs, Facebook, WebSockets, sitios, herramientas de captura, equipos y servicios locales o remotos, cuando realmente existan? ¿Qué sucede si fallan? |
| **Datos y persistencia** | ¿Qué almacena SQLite u otros sistemas, qué archivos y formatos usa Cortex, qué historiales son indispensables y cómo se restauran? |
| **Multimedia y recursos** | ¿Dónde se encuentran imágenes, videos, sonidos, overlays y archivos vinculados a OBS/Cortex? ¿Cuáles están fuera del repositorio? |
| **Flujos de uso real** | ¿Qué hace el usuario antes, durante y después de transmitir, qué automatiza y qué hace manualmente? ¿Qué funciones especiales usa con poca frecuencia? |
| **Rendimiento y estabilidad** | ¿Cuánta RAM, CPU, disco y latencia consume el conjunto real? ¿Qué errores, bloqueos, reinicios y pérdidas de conexión ocurren? ¿Cuál es el cuello de botella medido? |
| **Requisitos futuros** | ¿Qué desea conservar, simplificar, eliminar, corregir o incorporar el propietario? ¿Qué restricciones no aparecen en el código? |

**Método para obtener evidencia:** combinar lectura de archivos y configuraciones con registros existentes, observaciones autorizadas del funcionamiento y preguntas al propietario. Si es posible, cubrir un flujo representativo **antes → durante → después** de la transmisión. Una sola sesión no descarta funcionalidades utilizadas para eventos especiales, sorteos, emergencias u ocasiones excepcionales.

**Entrevista al propietario cuando la evidencia no alcance:** la primera IA debe hacer preguntas breves, agrupadas por prioridad, sobre funciones esporádicas, futuras, desactivadas temporalmente o que requieren intervención manual. Si no recibe respuesta, etiquetar el tema como **pendiente** y explicar cómo afecta las conclusiones; **no inferir que se puede eliminar**.

**Reglas de trazabilidad y seguridad:**

- Para cada afirmación importante registrar **fuente o evidencia**, contexto de observación, fecha si se conoce y **nivel de confianza**: confirmado, probable, hipótesis o no verificado. Indicar si el hallazgo proviene del código, OBS, Windows, datos, mediciones o confirmación del propietario.
- Separar estrictamente **configurado**, **implementado**, **observado en ejecución** y **declarado por el usuario**; pueden contradecirse.
- No etiquetar como «sin uso» una función solo porque no se activó durante una prueba o aparece deshabilitada. Para proponer eliminarla, justificar la recomendación y solicitar validación al propietario cuando corresponda.
- Trabajar **en modo de solo lectura** sobre código y configuraciones productivas. Usar copias o exportaciones autorizadas, sin editar OBS, iniciar servicios inesperados ni manipular datos reales. Las observaciones y mediciones durante una emisión deben ser no intrusivas y consensuadas.
- No incluir tokens, claves de streaming, contraseñas, URLs privadas ni datos personales innecesarios en los `.md`; referenciar evidencias de forma segura.
- Si faltan accesos a OBS, Windows, datos, dispositivos o registros, documentar **exactamente qué no se pudo auditar** y el impacto de ese vacío. No declarar «contexto completo» si quedan ausencias sustanciales.

**Criterio de salida de la fase:** la primera IA debe entregar un **mapa verificable del ecosistema**, una **especificación funcional de Cortex 2.0 independiente de cualquier lenguaje**, la lista de dudas y riesgos pendientes, y una **recomendación tecnológica fundamentada**. Si faltan evidencias críticas, podrá emitir una recomendación **provisional y condicionada**, pero no afirmar que ya existe fundamento suficiente para empezar la reconstrucción. La IA 2 deberá comprobar este criterio antes de recomendar que Codex (IA 3) comience.

### 13.2. Aportar observaciones y mejoras propias, más allá de este documento

La primera IA **no debe limitarse a validar Go o repetir estas sugerencias**. Debe aportar ideas nuevas basadas en lo que realmente encuentre:

- Detectar duplicaciones, dependencias o servicios prescindibles, acoplamiento, tareas innecesarias, configuraciones confusas y posibles puntos de fallo.
- Identificar oportunidades concretas para mejorar **rendimiento, RAM, CPU, estabilidad, velocidad de inicio, seguridad, confiabilidad, mantenibilidad y experiencia de uso**.
- Recomendar funciones que conviene conservar tal cual, simplificar, rediseñar, fusionar o retirar, sin eliminar comportamientos valiosos o requisitos ocultos.
- Considerar alternativas de arquitectura y distribución si el **monolito modular** no resulta lo mejor para las condiciones reales.
- Separar mejoras **imprescindibles**, **convenientes** y **opcionales**, indicando **beneficio esperado, complejidad, riesgos y evidencia**. Evitar ideas decorativas o funcionalidades no solicitadas.
- Revisar especialmente el impacto sobre OBS en transmisión, SQLite, conexiones y reconexiones, eventos, dispositivos y herramientas auxiliares, si forman parte de la versión real. **Usar la auditoría de OBS y el uso real para detectar simplificaciones**, no solo el código antiguo.
- No dar por garantizada una mejora de rendimiento sin medición o prueba representativa.

### 13.3. Tomar y fundamentar una decisión sobre el lenguaje principal

La primera IA deberá **elegir una opción principal concreta**, no entregar únicamente una lista neutral de candidatos. Debe razonar sobre las necesidades verificadas de Cortex y explicar:

1. **Lenguaje principal seleccionado** y por qué es el más conveniente para **reconstruir Cortex desde cero con Codex**.
2. Comparación justa con **Go, C#/.NET, Rust, PHP y el lenguaje/plataforma actual**; considerar **C++** cuando una integración nativa con OBS o Windows lo justifique.
3. Cómo resuelve integración con OBS, funcionamiento en Windows, concurrencia, procesos persistentes, interfaz, SQLite, captura/OCR y otras dependencias **efectivamente usadas**.
4. Qué ofrece en rendimiento y consumo de recursos, y cuáles de esas afirmaciones están **medidas** frente a las que siguen siendo hipótesis.
5. Facilidad para compilar, ejecutar pruebas, depurar, distribuir, actualizar y mantener el proyecto mediante Codex; viabilidad de un ejecutable principal y dependencias adicionales.
6. Por qué **descarta los competidores** y cuáles son los compromisos o inconvenientes del lenguaje ganador.
7. Si conviene un lenguaje único o una excepción puntual para un componente nativo, justificar el costo de un segundo lenguaje; evitar mezclas sin necesidad.
8. Qué evidencia o pruebas podrían **cambiar la recomendación**, y qué debe validarse antes de comprometer la reconstrucción.

**Resultado exigido:** una **decisión técnica recomendada, explícita y defendida**, con un nivel de confianza y condiciones de validación. Si no dispone de acceso a ejecutar pruebas, igualmente debe proponer el candidato más adecuado conforme al análisis disponible, **sin fingir mediciones ni declarar la decisión aprobada**. La decisión pasa después a la IA 2 para revisión independiente y al propietario para autorización; Codex (IA 3) recibirá la decisión aprobada.

### 13.4. Diseñar el nuevo Cortex sin arrastrar complejidad histórica

- Proponer una arquitectura y estructura conceptual **simple y coherente**, con módulos internos delimitados por función real.
- Proponer qué componentes se mantienen en el nuevo diseño y cuáles **no se reconstruyen**, explicando el porqué y el efecto sobre usuarios y datos.
- Establecer contratos y flujos entre módulos, gestión de estado, tratamiento de errores, almacenamiento y supervisión de procesos externos solo hasta el nivel necesario para planificar.
- Identificar requisitos de rendimiento, pruebas durante transmisiones, compatibilidad de datos, instalación, respaldo y reversión.
- No confundir «empezar de cero» con descartar funciones útiles, crear sistemas genéricos de plugins o eliminar dependencias que siguen siendo necesarias.
- Preparar una **hoja de ruta por fases comprobables** que Codex pueda ejecutar después de recibir aprobación.

## 14. Documentación `.md` que debe crear la primera IA

El propietario **asignará una carpeta específica**. La primera IA debe **guardar allí sus propios archivos Markdown**, organizados para que cualquier otra IA —incluido este asistente— pueda comprender el Cortex actual y evaluar la propuesta **sin depender del historial de conversaciones**.

### 14.1. Reglas de documentación

- **Esperar a conocer la carpeta asignada:** no inventar una ruta ni dispersar documentación por el repositorio.
- Crear o actualizar **solamente documentos `.md` dentro de esa carpeta**, salvo una instrucción posterior y explícita del propietario.
- Mantener una **portada o índice** con el orden recomendado de lectura, fecha, versión, estado de cada archivo y relaciones entre documentos.
- Incorporar una **matriz de cobertura de auditoría**: área revisada, alcance, fuentes, estado (completo/parcial/no accesible), hallazgos principales y acciones para completar el contexto.
- Escribir **en español**, con títulos claros, párrafos cortos y tablas o listas cuando ayuden. Priorizar información verificable sobre documentos excesivamente largos.
- **No incluir código de programación** en los documentos de planificación; describir comportamientos, arquitectura, criterios y decisiones en lenguaje natural.
- Referenciar rutas y componentes reales del repositorio cuando sea posible, y explicar la fuente de cada conclusión importante. Evitar exponer secretos, tokens y datos personales.
- Marcar expresamente: **Confirmado**, **Probable**, **Hipótesis**, **Recomendación**, **Pendiente de validar** y **Decisión propuesta**. Añadir **fuente de evidencia, fecha/contexto cuando exista y nivel de confianza**; no presentar opiniones como hechos.
- Evitar información duplicada: el índice debe enlazar a documentos específicos, no reproducirlos íntegramente.
- Mantener **registro de cambios y decisiones**, con fecha, motivos, alternativas y cuestiones abiertas. Si una IA revisa una propuesta, preservar el razonamiento anterior o su historial.
- No proclamar pruebas realizadas cuando no se ejecutaron. Anotar los bloqueos concretos de acceso o entorno.
- Conservar la documentación comprensible y reutilizable por IA 2 y Codex (IA 3), incluso en sesiones distintas.

### 14.2. Entregables sugeridos dentro de la carpeta asignada

Los nombres siguientes son **sugerencias organizativas**, adaptables a la dimensión real de Cortex. Unir documentos pequeños si así se simplifica la lectura; separar documentos extensos si mejora la consulta.

| Archivo Markdown sugerido | Contenido mínimo |
|---|---|
| **00-INDICE.md** | Resumen ejecutivo, mapa de documentos, estado general, orden de lectura y pendientes prioritarios. |
| **01-AUDITORIA-ACTUAL.md** | Tecnologías, estructura real, módulos, dependencias, flujos, integraciones y hallazgos sustentados en el repositorio. |
| **02-INVENTARIO-FUNCIONAL.md** | Qué hace Cortex hoy, qué está terminado, qué falta, qué se usa y qué comportamientos deben conservarse. |
| **02A-AUDITORIA-OBS.md** | Escenas, perfiles, fuentes, plugins, scripts, multimedia y dependencias relevantes de OBS; alcance del acceso y diferencias entre configuración y uso efectivo. |
| **02B-MATRIZ-CORTEX-OBS.md** | Trazabilidad entre función de Cortex, acción o recurso en OBS, disparador, frecuencia/contexto de uso, evidencia, prioridad, dudas y propuesta de conservar/rediseñar/retirar. |
| **02C-AUDITORIA-ENTORNO-WINDOWS.md** | Servicios, procesos, programas auxiliares, dispositivos, permisos, instalación y dependencias de funcionamiento. |
| **02D-MAPA-INTEGRACIONES.md** | Inventario y relación de APIs, conexiones, herramientas, sistemas externos y situaciones de desconexión. |
| **02E-INVENTARIO-DATOS-Y-RECURSOS.md** | Bases de datos, archivos, historiales, configuraciones, multimedia, respaldos y rutas necesarias, sin divulgar secretos. |
| **02F-FLUJOS-OPERATIVOS-REALES.md** | Flujos antes/durante/después del directo, tareas manuales, automatizaciones, usos ocasionales y evidencia disponible. |
| **02G-RENDIMIENTO-Y-ESTABILIDAD.md** | Línea base de RAM/CPU/latencia, errores, logs, mediciones reproducibles y carencias de observación. |
| **02H-REQUISITOS-CORTEX-2.md** | Especificación funcional independiente del lenguaje: conservar, mejorar, retirar previa aprobación o incorporar; criterios de aceptación. |
| **02I-COBERTURA-Y-PREGUNTAS.md** | Matriz de cobertura de auditoría, fuentes y niveles de confianza, bloqueos y preguntas priorizadas para el propietario. |
| **03-PROBLEMAS-Y-MEJORAS.md** | Problemas detectados, nuevas ideas propuestas por la IA, prioridades, riesgos y beneficios esperados. |
| **04-DECISION-DE-LENGUAJE.md** | **Lenguaje ganador**, motivos, comparación de alternativas, razones para descartar otras opciones, evidencia y validaciones pendientes. |
| **05-ARQUITECTURA-PROPUESTA.md** | Arquitectura candidata, división interna, comunicación, almacenamiento, herramientas externas y límites del diseño. |
| **06-PLAN-DE-RECONSTRUCCION.md** | Fases para Codex, orden de trabajo, dependencias, equivalencia funcional, pruebas, criterios de aceptación y marcha atrás. |
| **07-RIESGOS-Y-PENDIENTES.md** | Riesgos, incógnitas, preguntas para el propietario, hipótesis y decisiones que necesitan aprobación. |
| **08-TRASPASO-ENTRE-IA.md** | Resumen listo para IA 2 y Codex (IA 3): decisiones propuestas, evidencia, limitaciones, instrucciones de revisión y próximos pasos autorizados. |
| **09-DECISIONES-Y-CAMBIOS.md** | Historial de versiones documentales, razonamientos, cambios y cuestiones resueltas. |
| **10-ESTADO-Y-APROBACIONES.md** | Fuente de verdad: propuestas vigentes, revisión de IA 2, decisiones del propietario, estado de fases y qué puede o no puede comenzar Codex. |

**Entregable indispensable:** que una IA externa pueda abrir **`00-INDICE.md`**, encontrar la documentación relevante y entender **qué existe, qué se propone, qué se decidió recomendar, por qué y qué está pendiente**, sin volver a investigar todo desde cero.

### 14.3. Informe final de traspaso obligatorio de la primera IA

**Antes de cerrar su trabajo, IA 1 debe crear y entregar `08-TRASPASO-ENTRE-IA.md` como informe final de traspaso.** No alcanza con haber creado los demás archivos: este documento debe permitir que IA 2 comprenda el estado de Cortex 2.0, identifique las fuentes y continúe la revisión sin depender de chats anteriores ni tener que repetir toda la investigación.

El informe debe explicar, de manera **breve, ordenada y con enlaces a los `.md` de respaldo**:

1. **Qué descubrió:** síntesis de la auditoría de Cortex, OBS, Windows, integraciones, datos, recursos, rendimiento y uso real; qué se verificó, qué no pudo verificarse y con qué evidencia.
2. **Qué lenguaje eligió y por qué:** recomendación principal, motivos concretos, alternativas comparadas, ventajas y desventajas, pruebas disponibles y validaciones pendientes. Distinguir la decisión propuesta de una aprobación del propietario.
3. **Qué mejoras recomienda:** problemas detectados, simplificaciones posibles, nuevas ideas y prioridades, diferenciando necesidades confirmadas de propuestas opcionales.
4. **Qué funcionalidades deben conservarse:** inventario resumido de funciones y comportamientos esenciales del Cortex actual, con referencias a los documentos donde se detallan y se vinculan con OBS y el uso real.
5. **Qué decisiones siguen pendientes:** preguntas para el propietario, riesgos abiertos, información ausente, hipótesis y cuestiones que podrían cambiar la arquitectura o el lenguaje propuestos.
6. **Qué debe revisar exactamente IA 2:** lista priorizada de conclusiones por contrastar, evidencia que necesita consultar, pruebas faltantes, supuestos débiles y criterios para aceptar, condicionar o rechazar la propuesta.

El informe también debe indicar **la carpeta de documentación utilizada, el índice principal, los documentos entregados y su estado**, además de confirmar que **IA 1 no modificó el código de Cortex, OBS ni inició la reconstrucción**.

**Criterio de cierre:** IA 1 solo podrá declarar completado su encargo cuando este informe exista, sea coherente con los demás `.md` y deje claro qué sabe, qué propone y qué queda por resolver. Si falta información crítica, deberá dejarlo expresamente indicado; no presentará la auditoría como completa ni el proyecto como aprobado.

## 15. Responsabilidad de la segunda IA: contraste y mejora

La segunda IA debe revisar **tanto el proyecto vigente como la documentación creada por la primera**, sin asumir que la primera acertó automáticamente.

1. Verificar que la auditoría cubre el código y las funciones, las actualizaciones relevantes, **OBS, Windows, integraciones, datos, recursos, rendimiento y uso real**; comprobar la matriz de cobertura y los faltantes de evidencia.
2. Comprobar la **elección del lenguaje** y sus motivos; ratificarla o proponer otra con evidencia, no por preferencias personales.
3. Revisar las mejoras recomendadas, la arquitectura, los módulos y la eliminación de complejidad; detectar sobrediseño o riesgos ignorados.
4. Evaluar conservación de funcionalidades, datos, configuraciones, dependencias y comportamiento durante transmisiones con OBS.
5. Añadir alternativas, riesgos, pruebas o prioridades nuevas cuando encuentre motivos reales.
6. Documentar qué acepta, cambia o rechaza de los entregables de la primera IA y **por qué**.
7. Actualizar o complementar los `.md` de manera **trazable**, únicamente si cuenta con permisos sobre la carpeta asignada; evitar borrar evidencia y razonamientos previos.
8. Preparar un **dictamen final de revisión** para el propietario y un traspaso limpio para Codex (IA 3) si el plan es autorizado.

La segunda IA **no implementa** la nueva versión ni considera aprobado el lenguaje por el solo hecho de haberlo revisado.

## 16. Dictamen de IA 2, autorización del propietario y ejecución de Codex (IA 3)

La **IA 2** debe concluir su evaluación independiente con uno de estos resultados: **recomendación favorable**, **recomendación favorable con condiciones**, **solicitud de correcciones/pruebas** o **no recomendación**. Debe justificarlo con evidencia, señalar los riesgos y dejar explícito qué cuestiones siguen abiertas.

En particular, IA 2 debe comprobar:

- Que el inventario funcional refleja el **Cortex actual y su uso real con OBS**, no solo hipótesis o archivos antiguos.
- Que el lenguaje propuesto por IA 1 está justificado frente a alternativas viables y que no se inventaron mediciones.
- Que el nuevo diseño es tan sencillo como sea posible, sin omitir funciones importantes ni incorporar dependencias injustificadas.
- Que existen criterios de aceptación, pruebas con OBS, preservación de datos, seguridad de control remoto, instalación y plan de reversión.
- Que los documentos son consistentes y que Codex puede seguir el plan aprobado sin interpretar decisiones contradictorias.

**El propietario**, no la IA 2, decide si autoriza que **Codex (IA 3)** comience. Codex implementa por hitos, registra lo realizado y entrega cada hito a IA 2 para revisión. La sustitución del Cortex anterior exige una **nueva autorización expresa** del propietario tras verificar el funcionamiento con OBS.

No se necesita otra IA fija. Una opinión adicional solo se solicitaría si surge una controversia importante o un riesgo que justifique una tercera revisión independiente.

## 17. Decisiones abiertas que deben registrarse sin inventar respuestas

- Qué lenguaje principal **propone la primera IA** tras auditar Cortex y con qué evidencia; si la IA 2 está de acuerdo.
- Si un monolito modular sigue siendo la arquitectura más simple para los requisitos constatados.
- Qué funciones deben conservarse, corregirse, omitirse o replantearse, según evidencia contrastada entre Cortex y OBS.
- Qué comunicaciones con OBS requieren WebSocket, plugins nativos o herramientas auxiliares.
- Qué interfaz, persistencia y método de distribución son más adecuados para Windows.
- Qué datos requieren importación, respaldo, protección y restauración.
- Cuáles son los requisitos verificables de RAM, CPU, latencia, estabilidad y comportamiento sin conexión.
- Qué pruebas son automáticas y cuáles necesitan OBS, Windows o dispositivos reales.
- Qué colecciones, perfiles, plugins, fuentes, scripts, recursos externos y situaciones de uso real de OBS se pudieron verificar; qué información debe aportar el propietario y qué datos requieren protección.
- Qué áreas de **Windows, servicios, integraciones, bases de datos, recursos multimedia, rendimiento y estabilidad** siguen sin auditar; qué evidencias o entrevistas del propietario son indispensables para cerrar esas brechas.
- Qué ruta de trabajo documental asignará el propietario y qué permisos de escritura tiene la primera IA.
- Qué fases, condiciones y aprobaciones debe cumplir Codex antes de implementar y antes de reemplazar Cortex actual.

## 18. Referencias técnicas para comprobar

- **Código fuente de OBS Studio:** https://github.com/obsproject/obs-studio
- **Guía oficial para desarrolladores OBS (plugins, scripts y WebSocket):** https://obsproject.com/kb/developer-guide
- **OBS: colecciones de escenas:** https://obsproject.com/kb/scene-collections
- **OBS: perfiles:** https://obsproject.com/kb/profiles
- **OBS: fuentes:** https://obsproject.com/kb/sources-guide
- **Documentación oficial de plugins OBS:** https://docs.obsproject.com/plugins
- **Documentación oficial de Go:** https://go.dev/doc/
- **Documentación de OpenAI Codex:** https://developers.openai.com/codex/
- **Documentación oficial de PHP:** https://www.php.net/docs.php
- **Documentación oficial de .NET:** https://learn.microsoft.com/dotnet/
- **Documentación oficial de Rust:** https://www.rust-lang.org/learn

Estas referencias aportan contexto sobre tecnologías, **no prueban todavía qué conviene a Cortex**.

---

## Síntesis de traspaso para IA 1, IA 2 y Codex (IA 3)

**Si estás leyendo este archivo como primera IA: tu rol es IA 1 — investigación y arquitectura.** Empezá por las «INSTRUCCIONES DE INICIO» y luego aplicá las secciones 13 y 14. Solicitá la carpeta que asignará el propietario si todavía no fue indicada; allí escribirás tu documentación `.md`. **No programes ni modifiques Cortex actual.**

**Objetivo:** evaluar una **reconstrucción limpia de Cortex desde cero**, basada en el Cortex casi terminado, **OBS real, Windows, integraciones, datos, uso efectivo, rendimiento y requisitos futuros**. No asumir que Go está decidido ni que el programa actual contiene todos los requisitos conocidos por el propietario.

**IA 1:** audita íntegramente lo accesible, distingue evidencias de hipótesis, hace preguntas cuando la información falta, propone mejoras originales, define los requisitos y una arquitectura simple, **elige y justifica un lenguaje principal**, y entrega Markdown organizado y rastreable.

**IA 2:** verifica de forma independiente las evidencias, omisiones, decisiones tecnológicas y riesgos; propone correcciones, emite dictamen al propietario y después revisa hitos de Codex.

**Propietario:** aprueba o rechaza el plan y autoriza cada cambio de etapa crítico.

**Codex (IA 3):** solamente después de autorización, desarrolla Cortex 2.0 por etapas en un entorno separado, prueba los requisitos aprobados y solicita revisión de IA 2. No sustituye ni desinstala Cortex actual por iniciativa propia.

**Contexto de control remoto:** se desea que un Cortex principal conectado a OBS pueda controlarse desde otra PC por LAN/Tailscale, sin altas de usuarios ni login adicional; priorizar un panel remoto privado, condicionado a controles de acceso a nivel de red, y evaluar la solución mínima necesaria.

**Regla central:** documentación y evidencia antes que código; aprobación antes que implementación; simplicidad demostrable antes que modas tecnológicas. **Tres roles bastan: IA 1 analiza, IA 2 revisa, Codex (IA 3) construye.**
