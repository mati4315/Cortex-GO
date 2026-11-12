# Requisitos independientes del lenguaje y decisión tecnológica

**Fecha:** 12 de noviembre de 2026 (Australia/Brisbane)  
**Estado de decisión:** Go elegido por el propietario; el primer corte está implementado y probado. La estabilidad del producto completo todavía no está medida.  
**Confianza:** media. Falta confirmar uso real, ver la configuración de escenas y medir un flujo representativo.

El propietario aclaró que busca **unificar la operación**, no imponer que cada dependencia sea un único proceso: iniciar Cortex una vez, administrar desde ahí los módulos y evitar lanzarlos o supervisarlos por separado; OBS sigue como aplicación principal aparte. También necesita confirmar la captura de dos cuentas de Facebook. La extensión actual usa un navegador, por lo que cualquier auxiliar debe quedar gestionado y recuperado por Cortex. El propietario eligió Go para el núcleo después de la recomendación IA 2; la elección no equivale a una medición de estabilidad ni de rendimiento.

## Recomendación principal

**Go será el lenguaje principal del núcleo/backend de Cortex 2.0.** El programa nuevo para Windows coordinará los módulos, expondrá el panel central y se conectará a OBS por WebSocket. Es una reconstrucción desde requisitos, no una traducción del proyecto Node actual. El panel web y la extensión/adaptador de navegador serán código nuevo con tecnologías de navegador; no son una migración del backend.

El propietario confirma que todos los módulos actuales salvo SocialStream Ninja deben conservar su capacidad funcional. Esa amplitud refuerza la necesidad de agruparlos por responsabilidades internas y compartir panel, estado y datos; no exige mantener un proceso separado por módulo. La razón principal no es una promesa de que Go consuma menos recursos. El sistema observado cumple principalmente tareas persistentes de servidor: endpoints HTTP, WebSocket, colas, servicios locales, integraciones externas y trabajo de I/O concurrente. El panel remoto se entrega al navegador y no necesita una interfaz nativa de Windows. La documentación oficial de Go describe paquetes y concurrencia como parte del lenguaje/runtime, y documenta que go build compila paquetes y dependencias a un ejecutable. Esto encaja con un backend sencillo de desplegar como unidad; no prueba consumo ni rendimiento de Cortex.

La integración documentada con OBS usa OBS WebSocket. No apareció evidencia de que Cortex necesite ejecutar dentro del proceso OBS. La documentación oficial de OBS trata los plugins nativos como módulos dinámicos que implementan fuentes, salidas, codificadores u otros objetos; no hay requisito actual comprobado que justifique ese acoplamiento. Si luego aparece una función que WebSocket no puede cubrir, C++ puede ser una excepción limitada a ese plugin.

## Comparación

| Opción | Ajuste al caso observado | Compromisos y motivo de decisión |
|---|---|---|
| **Go — recomendado** | Núcleo de servicios Windows y red, WebSocket con OBS, endpoints locales, gestión de concurrencia y un ejecutable compilado. Su organización en paquetes internos puede ayudar a sustituir muchos servicios sin crear microservicios. | Aún requiere elegir y comprobar librerías para SQLite, Spotify, OAuth, procesos Windows y UI. No hay mediciones comparativas; el recolector de basura no garantiza consumo bajo. |
| **C# / .NET — segunda opción** | Muy buen candidato en producto Windows, diagnósticos y herramientas de escritorio. También puede publicar como archivo único o Native AOT. | No se necesita una UI nativa según la evidencia disponible; el producto usa panel web remoto. Native AOT tiene limitaciones de compatibilidad de funciones/librerías y exige validación de cada dependencia. Si aparecen necesidades fuertes de integración nativa Windows, puede superar a Go. |
| **Rust** | Control de memoria sin recolector de basura y seguridad de memoria verificada por el compilador. | Mayor complejidad para el dueño y el ciclo diario de cambios de un producto centrado en I/O y paneles. No hay evidencia de latencia o presión de memoria que justifique ese costo. Reconsiderar solo con requisitos medidos. |
| **PHP** | Puede ejecutar programas CLI independientes del servidor web y tiene afinidad histórica del propietario. | En este proyecto el núcleo debe alojar servicios persistentes y coordinar red/OBS en Windows; despliegue, extensiones y concurrencia requieren validar más piezas. Codex implementaría principalmente, pero la revisión y operación del propietario siguen siendo importantes. No lo descarto por falta de capacidad; lo descarto por ajuste inferior al objetivo de despliegue y concurrencia simple. |
| **Node.js / TypeScript actual** | Ya tiene servicios, navegador, clientes WebSocket y paneles implementados. La migración gradual sería la alternativa de menor riesgo técnico. | El propietario pidió empezar de cero con otro lenguaje. El árbol actual tiene muchas unidades y responsabilidades; la reconstrucción puede simplificar, pero no por el mero cambio de runtime. Node sigue siendo una referencia funcional y de riesgos, no código para portar. |
| **C++** | Apropiado si una capacidad necesita un plugin nativo que se ejecuta dentro de OBS o acceso directo a libobs. | No se encontró esa necesidad; para control externo aumenta el costo de mantenimiento sin beneficio demostrado. Reservarlo para la excepción justificada, no para el núcleo. |

## ¿Hay una opción claramente mejor fuera de la lista inicial?

Contrasté dos familias razonables adicionales. No encontré una que mejore el ajuste global del caso actual por encima de Go:

| Candidato adicional | Qué ofrece | Evaluación para Cortex |
|---|---|---|
| **Java / Kotlin sobre JVM** | Ecosistema maduro para servicios. Oracle documenta que jpackage puede crear un paquete Windows con un runtime propio para la aplicación. | Es viable, pero implica distribuir una imagen de runtime y producir paquetes por plataforma. No aparece una necesidad JVM específica que compense esa capa frente a un backend Go. Kotlin/JVM comparte la misma base de ejecución, así que tampoco aporta una ventaja clara por sí solo. |
| **Python** | Útil para automatización, procesamiento de datos, OCR e integración de modelos; Windows incluye una distribución embebible oficial. | Sería más atractivo si Cortex ejecutara OCR/IA local o procesamiento científico pesado. La distribución embebible sigue siendo un entorno Python que el paquete debe reunir con sus dependencias y runtime de C de Microsoft; no simplifica el despliegue de este servidor frente a Go. |
| **TypeScript con otro runtime JavaScript** | Puede reutilizar bibliotecas del ecosistema web. | No es un lenguaje distinto del núcleo actual en el sentido que pidió el propietario y no resuelve por sí solo la fragmentación de servicios. Mantenerlo como única base solo se justificaría si la reescritura en Go encarece mucho la captura o el mantenimiento. |

No ampliaría más la lista por ahora. Lenguajes especializados como Zig, Nim o Elixir no resuelven una necesidad observada que justifique subir el costo de aprendizaje, integración y mantenimiento. La prueba aprobada podría abrir de nuevo la comparación si descubre un requisito nuevo.

**Conclusión afinada:** Go es el mejor candidato identificado para prototipar como lenguaje principal; no se puede afirmar que sea un ganador medido ni que sea universalmente mejor. C#/.NET es la alternativa más fuerte para Windows; Java/Kotlin y Python son viables pero no superiores con la evidencia presente.

**Resultado de la prueba inicial:** `modernc.org/sqlite` se integró como driver puro de Go y el corte local creó, cerró y reabrió una base sintética en Windows. La aplicación mantiene un solo proceso escritor y no usa la base del sistema viejo. Antes de usarla con datos reales todavía corresponde definir respaldos, retención, migraciones futuras y pruebas prolongadas de recuperación. [Documentación Go sobre drivers](https://go.dev/doc/database/open-handle), [referencia `database/sql`](https://pkg.go.dev/database/sql), [driver modernc SQLite](https://pkg.go.dev/modernc.org/sqlite).

## Interfaz y componentes que necesariamente viven en el navegador

Go sería el lenguaje principal del backend. El panel y la extensión de captura se vuelven a diseñar y escribir para las APIs del navegador; sus documentos, HTML/CSS y JavaScript serían código nuevo. No se copiarían los archivos actuales. La extensión de navegador requiere código compatible con el navegador y no convierte el backend en un proyecto Node.

No recomiendo que la interfaz sea una aplicación nativa separada mientras el objetivo sea abrir el panel desde otra PC por LAN/Tailscale. Un panel web servido por Cortex evita distribuir un segundo controlador.

## Requisitos funcionales que no dependen del lenguaje

1. Un Cortex principal conserva el estado y coordina los módulos aceptados; el panel remoto no inicia una segunda copia completa.
2. Conexión autenticada y observable con OBS WebSocket; reconexión acotada, estado explícito de desconexión y confirmación de acciones.
3. Entradas/captura y eventos con identificadores de deduplicación; trazas técnicas sin texto ni identidad privada cuando no se necesiten.
4. Operaciones de Spotify que confirman dispositivo, URI y reproducción real antes de comunicar éxito. Modo de prueba aislado para que no se escriba en historial ni se mezclen pedidos reales.
5. Historial, participantes, memoria y respaldos con responsables claros, retención entendible, corrección/borrado cuando corresponda y recuperación comprobable.
6. Perfiles y preparación del directo que muestren qué se aplicará, validen conexiones y no declaren listo un requisito que depende de una comprobación visual humana.
7. Módulos con límites definidos, errores visibles y cierre ordenado; colas acotadas y control de repetición cuando la acción pueda otorgar premios o cambiar OBS.
8. Panel privado accesible desde dispositivos de confianza; mantener claves de Spotify, OBS, Firebase y proveedores IA solo en el servidor.
9. Comportamiento verificable con Windows, OBS y la división real de PCs; no aceptar promesas de ahorro de recursos sin medidas comparables.

## Validación antes de comprometer la reconstrucción

La elección de Go no habilita por sí sola una reescritura completa ni demuestra que el servidor no volverá a cerrarse. Antes de reconstruir todos los módulos, conviene autorizar un primer corte técnico pequeño en Go: un único inicio, panel central, supervisor/recuperación del proceso, estados separados para dos fuentes de comentarios de ensayo, reconexión sin abrir módulos manualmente, OBS WebSocket en solo lectura y SQLite temporal/sintética. Medir arranque, recuperación, recursos y claridad de errores. Cualquier validación con Facebook real debe ser autorizada y observada por el propietario, sin guardar claves en código/logs ni modificar escenas.

Cambiaría la recomendación a C# si el requisito termina siendo UI/servicios nativos de Windows, si Go necesita demasiados puentes externos para supervisar el entorno real, o si el prototipo muestra desventajas medibles. Cambiaría a Rust solo por un límite comprobado de recursos/latencia. Mantendría Node si el costo de rehacer la captura o la integración del navegador supera el beneficio operativo de cambiar, aunque eso contradiga el deseo de otro lenguaje y debería discutirse expresamente.

## Referencias técnicas primarias consultadas

- [Go specification](https://go.dev/ref/spec) — paquetes, runtime, recolección de basura y concurrencia.
- [Compilar e instalar con Go](https://go.dev/doc/tutorial/compile-install) — diferencia entre go run y generar un ejecutable con go build.
- [Effective Go: concurrency](https://go.dev/doc/effective_go#concurrency) — goroutines, comunicación y límites entre concurrencia y paralelismo.
- [Publicación de aplicaciones .NET](https://learn.microsoft.com/dotnet/core/deploying/) y [Native AOT](https://learn.microsoft.com/dotnet/core/deploying/native-aot/) — opciones de archivo único y límites de compatibilidad.
- [PHP CLI](https://www.php.net/commandline) — PHP puede ejecutar programas independientes por CLI; Windows distribuye php.exe.
- [Rust: ownership](https://doc.rust-lang.org/book/ch04-00-understanding-ownership.html) — seguridad de memoria sin recolector de basura.
- [Documentación OBS 33.0.1](https://docs.obsproject.com/) y [plugins OBS](https://docs.obsproject.com/plugins) — WebSocket/OBS API y módulos de plugins.- [Oracle jpackage JDK 27](https://docs.oracle.com/en/java/javase/27/docs/specs/man/jpackage.html) — empaquetado de aplicaciones Java con runtime propio.- [Distribución embebible de Python para Windows](https://docs.python.org/3.12/using/windows.html#the-embeddable-package) — forma y dependencias de ese paquete.




