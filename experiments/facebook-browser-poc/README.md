# Prueba técnica: dos sesiones de Facebook

Este prototipo implementa una captura externa aislada para validar la guía. Abre dos ventanas del mismo navegador Chromium con carpetas de perfil distintas y observa la vista de comentarios de Live Producer. Desde la línea base, envía los comentarios nuevos a la API local de Cortex Go, que los deduplica y guarda en la SQLite local.

La prueba usa Edge o Brave instalados en Windows y el protocolo local de DevTools. Sirve para validar inicio de sesión manual, 2FA, persistencia, separación de perfiles y detección de cambios del DOM con el navegador actualizado del equipo. No integra Facebook al ejecutable principal de Cortex Go ni decide todavía el motor integrado definitivo. El empaquetado de CEF dentro de una ventana de Cortex sigue pendiente; esta prueba con navegador externo valida el flujo de captura antes de esa decisión.

## Ejecutar

Inicia Cortex Go en la misma PC y luego, desde esta carpeta, inicia el capturador:

```powershell
go mod tidy
go run . -browser edge
```

La API predeterminada es `http://127.0.0.1:17845`. Se puede cambiar con `-cortex-url`, pero por seguridad solo se aceptan direcciones de loopback.

También se puede elegir Brave:

```powershell
go run . -browser brave
```

El programa abre dos ventanas con perfiles independientes y las titula **Cortex Go — Cuenta 1** y **Cortex Go — Cuenta 2**. Inicia sesión manualmente en la cuenta que corresponda a cada ventana, completa 2FA si aparece y navega en esa misma pestaña a la vista de comentarios del directo. Comprueba visualmente qué cuenta quedó iniciada; el prototipo no puede verificar por sí solo la identidad de Facebook. El monitor muestra si encontró la ruta, las filas visibles y la hora del último comentario nuevo. Solo se envían los comentarios que aparecen después de la línea base.

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

La comprobación de 2FA, separación, persistencia de sesión y selectores requiere que el propietario autentique manualmente sus dos cuentas en la PC de escritorio. Tras reiniciar el prototipo, hay que confirmar que cada perfil conserva su propia sesión, que solo los comentarios nuevos se guardan en la cuenta correcta y que el backend deduplica filas re-renderizadas.
