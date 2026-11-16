# Captura de comentarios de Facebook con Brave

Este prototipo abre una única ventana de Brave por cada URL configurada, con perfiles locales independientes para Cuenta 1 y Cuenta 2. Observa la vista Comentarios de Live Producer y entrega los comentarios nuevos al historial de Cortex Go, donde se deduplican y guardan en la SQLite local. No usa SocialStream Ninja, la extensión antigua ni la API oficial de Meta.

## Iniciar

1. Abre Cortex Go en la PC donde corre OBS.
2. Desde esta carpeta, ejecuta `go run .`. Pega una URL por cuenta y deja vacía la cuenta que no usarás.
3. Pega el enlace completo de Live Producer > Comentarios, por ejemplo `https://www.facebook.com/live/producer/dashboard/1234567890/COMMENTS/`.
4. Inicia sesión manualmente en cada ventana de Brave y confirma que cada perfil tenga la cuenta correcta. Cada perfil conserva sus cookies en `%LOCALAPPDATA%\CortexGo\FacebookBrowserPoC\brave\cuenta-1` o `cuenta-2`.
5. Deja la vista Comentarios abierta. En el historial, el indicador inferior debe mostrar `Captura activa (1/2)` o `Captura activa (2/2)`. Pasa el cursor por el indicador para ver filas visibles, comentarios reconocidos y errores por cuenta.
6. Publica un comentario nuevo después de iniciar la captura. Los comentarios que ya estaban visibles al iniciar se toman como línea base y no se importan de nuevo.

También se puede iniciar desde PowerShell:

```powershell
Push-Location .\experiments\facebook-browser-poc
go run .
Pop-Location
```

Para automatizar el ingreso de URL se pueden usar `go run . -url1 "URL" -url2 "URL"`; `-url2` puede omitirse. `-cortex-url` cambia el destino, pero por seguridad solo acepta `localhost`, `127.0.0.1` o `::1`.

## Qué muestra y cómo recuperarse

- El título de cada ventana indica Cuenta 1 o Cuenta 2.
- El panel diferencia capturador sin señal, Facebook abierto fuera de Comentarios y Comentarios activo. El estado deja de figurar activo si no recibe señal durante 15 segundos.
- El historial indica cuántas filas Facebook muestra y cuántas pudieron convertirse en comentarios. Si hay filas pero no se extraen, muestra un aviso en el indicador y en la consola.
- Si falla una cuenta, cierra el capturador con `Ctrl+C` e inicia de nuevo. El otro perfil y su sesión son independientes. No es necesario reiniciar todo Cortex salvo que también falle su panel o base local.
- Para volver a autorizar una cuenta, inicia sesión manualmente en la ventana correspondiente. No copies el perfil de Brave habitual ni los perfiles de captura entre equipos.

## Comportamiento y límites

- Se acepta únicamente una URL HTTPS de `facebook.com` con `/live/producer/dashboard/ID/COMMENTS/` y un ID numérico. El capturador vuelve a verificar dominio, ruta e ID antes de guardar.
- Hay como máximo una ventana/perfil persistente por cuenta. Dejar la segunda URL vacía abre solo una ventana.
- Brave puede mostrar su propio proceso auxiliar; Cortex solicita una sola ventana/pestaña por cuenta y no abre métricas u otras vistas.
- Se observa el DOM mediante `MutationObserver` y un escaneo de respaldo cada cinco segundos. La línea base evita guardar los comentarios existentes al iniciar. Se extrae el texto accesible de filas de la vista de comentarios, incluso si la propia publicación muestra la acción `Fijar` y no `Responder`.
- Cada comentario usa el ID visible si está disponible; si no, se deduplica con una huella del directo, participante/autor, texto y hora disponible. La huella puede colisionar si Facebook no expone IDs y dos mensajes idénticos no tienen hora distinguible.
- La conexión DevTools se enlaza a loopback. No se copian ni registran cookies, tokens o texto de comentarios en la consola. El historial SQLite sí conserva nombre y texto hasta su borrado manual.
- La captura depende de lo que Facebook carga en la sesión y de sus selectores del DOM. No puede garantizar comentarios filtrados, ocultos, eliminados o aún no cargados. Cambios de Facebook pueden exigir ajustar y volver a observar el extractor.
- Una captura activa confirma que el navegador informa su estado; no asegura por sí sola que Facebook esté entregando todo el chat. La validación final requiere un comentario real nuevo por cada cuenta que se quiera usar.
