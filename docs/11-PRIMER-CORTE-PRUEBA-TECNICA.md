# Primer corte técnico de Cortex 2.0

**Lenguaje del núcleo:** Go, elegido por el propietario.  
**Estado:** primer corte implementado y probado localmente; Cortex anterior sin cambios.  
**Propósito:** validar primero el inicio unificado, la recuperación del servidor y el estado de las conexiones, antes de reescribir los módulos completos.

## Qué construir primero

Una base mínima nueva, separada del sistema anterior, con:

1. **Un punto de inicio visible:** el propietario abre Cortex una vez. No aparecen ventanas de terminal o `.bat` por cada módulo.
2. **Supervisor que sobrevive al servidor:** si el proceso backend termina, el supervisor detecta la salida, lo reinicia con una espera limitada y deja un registro del motivo. La elección concreta entre proceso supervisor, servicio de Windows u otra opción se toma en este corte, después de comprobar el modo de operación más sencillo.
3. **Panel de estado único:** Cortex, SQLite sintética, OBS y dos fuentes de comentarios de prueba tienen estados independientes, último latido, último error y reconectar/reiniciar cuando corresponda.
4. **Persistencia sintética:** una base nueva de prueba registra eventos ficticios y confirma guardado/cierre. No se conecta ni se copia la base anterior.
5. **OBS solo de lectura:** consultar que OBS está conectado y mostrar estado; no cambiar escena, iniciar medios ni tocar colecciones.
6. **Captura representada en dos feeds simulados:** probar que cada cuenta/fuente tiene estado separado y que una desconexión no oculta la otra. La captura real de Facebook queda para un paso posterior controlado por el propietario.

El diseño debe mantener una sola experiencia de operación aunque el supervisor y el backend sean procesos distintos. No se fuerza que todas las dependencias se ejecuten dentro de un único proceso.

## Pruebas de aceptación del primer corte

| Caso | Acción | Resultado esperado |
|---|---|---|
| Inicio | Abrir Cortex desde el punto de inicio nuevo. | Aparece el panel central y cada componente indica estado; no se abren lanzadores separados por módulo. |
| Inicio repetido | Pulsar el acceso de Cortex otra vez mientras ya corre. | Se enfoca la instancia actual o muestra su estado; no crea dos servidores ni dos escritores de base. |
| Caída del backend | Detener de forma controlada el proceso backend de prueba. | El supervisor lo detecta, lo inicia otra vez con demora limitada y muestra que hubo un reinicio. |
| Fallo persistente | Simular un fallo que se repite. | No hay bucle de aperturas: los intentos se limitan, se registra la causa y el panel solicita intervención. |
| Datos sintéticos | Registrar un evento, reiniciar el backend y consultar el evento. | El registro persiste en la base nueva de prueba; no se lee ni modifica la base anterior. |
| Dos feeds | Desconectar solo el feed simulado A y luego volverlo a conectar. | A muestra su fallo y recuperación; B conserva su estado. |
| OBS desconectado | Detener OBS o su WebSocket durante la prueba. | El panel permanece disponible, indica OBS desconectado y recupera estado al volver OBS, sin cambiar escenas. |
| Cierre normal | Cerrar Cortex desde su control de salida. | No quedan escrituras pendientes, los procesos de prueba se cierran y una nueva apertura no encuentra instancias huérfanas. |
| Registro diagnóstico | Revisar los mensajes del corte. | Se identifica qué componente falló y cuándo, sin escribir tokens, claves ni comentarios reales. |

Los umbrales de tiempo/reintento y el hardware de referencia se acuerdan antes de ejecutar la prueba; no se inventan valores ni se prometen reducciones de CPU/RAM.

## Qué queda fuera de este primer corte

- Reescribir todos los módulos de Rulo, Spotify, ruleta, lotería, luces y Trigger Studio.
- Cambiar escenas, hotkeys, perfiles o ajustes de OBS.
- Conectar las cuentas reales, emitir comentarios reales o probar durante un directo.
- Conceder premios, lanzar bolilleros, reproducir música o publicar overlays reales.
- Importar datos antiguos, copiar secretos o reemplazar el `.bat` del sistema actual.

Después de aceptar este corte, el primer módulo real a integrar debería ser la captura de comentarios de Facebook en las dos cuentas, primero con validación observada por el propietario y con el mismo panel mostrando el estado de cada cuenta. Luego se prueba un flujo reversible de OBS/Trigger Studio antes de incorporar efectos monetarios o funciones de mayor impacto.

## Decisiones que el corte debe responder

- ¿Puede el propietario iniciar Cortex una sola vez y ver todos los estados desde un panel central?
- ¿Qué opción supervisa el proceso Go si este termina y cuál es el comportamiento cuando se reinicia varias veces seguidas?
- ¿Cómo se evita ejecutar dos instancias o perder/escribir dos veces los datos?
- ¿Qué debe mostrarse para distinguir “conectado” de “recibí un comentario hace poco” en cada feed?
- ¿La forma de empaquetado permite que la notebook abra el panel remoto sin instalar módulos ni copiar credenciales?

La prueba verifica viabilidad, no equivale a terminar Cortex 2.0. Cada módulo se incorpora después como capacidad interna con su estado, fallos, acciones seguras y criterio de aceptación.

## Resultado del primer corte implementado

- `CortexGo.exe` sirve un panel local y supervisa un proceso servidor interno. Abrir otra instancia usa el panel existente en vez de crear otro escritor de SQLite.
- El supervisor espera a que el servidor responda, lo revisa cada segundo y reinicia el proceso tras tres fallos consecutivos de salud o una salida inesperada. Usa esperas de 1, 2, 4 y 8 segundos y se detiene tras cinco reinicios automáticos consecutivos; el panel permite pedir un reinicio manual.
- El proceso servidor comprueba que el supervisor siga presente y se cierra al perder tres comprobaciones consecutivas. Esto reduce el riesgo de dejar un servidor huérfano si el supervisor se cierra abruptamente; el reinicio del supervisor mismo todavía requiere volver a abrir el ejecutable.
- Los feeds A y B pueden simular comentarios y desconectarse/reconectarse por separado. SQLite guarda los eventos sintéticos en `data/first-cut.sqlite`; no abre ni copia la base previa.
- La comprobación de OBS usa `obs-websocket` local y solo completa la identificación de sesión. No consulta escenas ni ejecuta operaciones. Si se configura autenticación, la clave puede recibirse en memoria desde `CORTEX_OBS_PASSWORD`; Cortex no la guarda ni la escribe en el registro.
- Las pruebas automatizadas verifican persistencia tras reabrir SQLite, independencia de feeds, saludo WebSocket de OBS sin suscripciones a eventos y límites de espera del supervisor.
- La prueba del ejecutable forzó el cierre del servidor hijo y confirmó recuperación, persistencia del evento, reinicio manual y cierre sin procesos huérfanos. También comprobó que abrir Cortex Go por segunda vez reutiliza el panel y que un origen HTTP externo no puede solicitar el reinicio.

### Límites y prueba siguiente

- Facebook real, Spotify, Rulo, sorteos, luces, Trigger Studio y escritura en OBS siguen fuera de este corte.
- El reinicio del servidor interno y el cierre ordenado se verificaron con el binario recién compilado en Windows; queda probar sesiones más largas y fallos repetidos hasta el límite de reintentos.
- Si se cierra el proceso supervisor entero, no hay servicio de Windows que lo vuelva a iniciar. Esa decisión se mantiene para una prueba posterior del arranque automático, con criterios de inicio de sesión y recuperación acordados.
- El panel está ligado a loopback. El control remoto desde la notebook no está abierto todavía y requerirá autenticación antes de habilitar acceso por red.

