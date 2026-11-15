# Prueba técnica: captura de comentarios de Facebook

Este prototipo implementa una captura externa aislada para validar la guía. Abre Brave por defecto con perfiles independientes y observa la vista de comentarios de Live Producer en la pestaña que inició. Desde la línea base, envía los comentarios nuevos a la API local de Cortex Go, que los deduplica y guarda en la SQLite local.

La prueba usa Brave instalado en Windows y el protocolo local de DevTools. Edge queda disponible como alternativa con `-browser edge`. Sirve para validar inicio de sesión manual, 2FA, persistencia, separación de perfiles y detección de cambios del DOM con el navegador actualizado del equipo. No integra Facebook al ejecutable principal de Cortex Go ni decide todavía el motor integrado definitivo. El empaquetado de CEF dentro de una ventana de Cortex sigue pendiente; esta prueba con navegador externo valida el flujo de captura antes de esa decisión.

## Ejecutar

Inicia Cortex Go en la misma PC y luego abre `FacebookBrowserPoC.exe` desde esta carpeta. El ejecutable abre Brave automáticamente. Para ejecutar Edge de forma explícita, inicia el programa con `-browser edge`.

```powershell
Push-Location .\experiments\facebook-browser-poc
.\FacebookBrowserPoC.exe -account 2
Pop-Location
```

El ejecutable se compila desde el código local con `go build -o .\FacebookBrowserPoC.exe .`; el `.exe` queda ignorado por Git.

`-account 2` abre solo la segunda cuenta (perfil `cuenta-2`); usa `-account 1` para la primera. Sin `-account`, abre ambas.

La API predeterminada es `http://127.0.0.1:17845`. Se puede cambiar con `-cortex-url`, pero por seguridad solo se aceptan direcciones de loopback.

El programa abre una ventana por cuenta seleccionada, con perfiles independientes titulados **Cortex Go — Cuenta 1** o **Cortex Go — Cuenta 2**. Inicia sesión manualmente, completa 2FA si aparece y navega en esa misma pestaña a la vista de comentarios del directo. Comprueba visualmente qué cuenta quedó iniciada; el prototipo no puede verificar por sí solo la identidad de Facebook. El monitor muestra si encontró la ruta, las filas visibles y la hora del último comentario nuevo. Solo se envían los comentarios que aparecen después de la línea base.

La sesión y las cookies se guardan en `%LOCALAPPDATA%\CortexGo\FacebookBrowserPoC\cuenta-1` y `cuenta-2`. El cierre normal con `Ctrl+C` pide el cierre ordenado de los navegadores. Las carpetas se conservan para comprobar la persistencia al volver a ejecutar el prototipo.

## Límites de esta prueba

- Mantiene un perfil independiente por cuenta y puerto DevTools enlazado solo a `127.0.0.1`.
- La línea base ignora las filas visibles al comenzar; después observa cambios con `MutationObserver` y una exploración cada cinco segundos.
- La consola no imprime nombres ni textos. El backend guarda nombre y texto de comentarios nuevos en la base local, de acuerdo con la retención manual configurada por el propietario; nunca se envían al repositorio ni a la notebook.
- El capturador recuerda las huellas DOM en memoria para no reenviar filas re-renderizadas. Cortex guarda el ID externo o una huella no reversible por cuenta/directo para evitar duplicados después de reiniciar.
- No se guardan URL de perfil, cookies ni tokens. La sesión de Facebook queda solo en los perfiles locales de Edge/Brave.
- Una detección demuestra que el navegador expuso cambios del DOM; no garantiza que Facebook haya cargado todos los comentarios.
- Las ventanas son visibles y se inician separadas del Cortex Go principal. Esta es una prueba aislada de navegador, no la experiencia final integrada.
- El prototipo no usa SocialStream Ninja, la extensión vieja, su servidor, sus tokens ni sus bases de datos.

## Resultado que falta observar

La comprobación de 2FA, separación, persistencia de sesión y selectores requiere que el propietario autentique manualmente cada perfil que vaya a usar en la PC de escritorio. Tras reiniciar el prototipo, hay que confirmar que cada perfil conserva su propia sesión, que solo los comentarios nuevos se guardan en la cuenta correcta y que el backend deduplica filas re-renderizadas.
