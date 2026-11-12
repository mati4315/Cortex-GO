# Cortex Go

Reconstrucción nueva de Cortex. El código anterior se conserva solo como referencia funcional: no se copia ni se migra.

## Estado

La base operativa está implementada en código: el lanzador recupera Cortex si el proceso principal termina inesperadamente; el servidor usa SQLite con esquema versionado, un registro general de eventos, deduplicación y borrado manual; el panel es PWA y recibe estado de dos perfiles de Facebook separados.

La captura Facebook es todavía un prototipo externo de Edge/Brave conectado al API local. La validación manual de login, 2FA, aislamiento y comentarios nuevos en la PC principal sigue pendiente. OBS se mantiene en solo lectura y los otros módulos aún no se integran.

## Estructura

- `cmd/cortex-go/`: punto de entrada del ejecutable.
- `cmd/cortex-go/web/`: panel central, manifiesto, iconos y service worker PWA incluidos dentro del ejecutable.
- `experiments/facebook-browser-poc/`: prototipo aislado de captura de Facebook con dos perfiles separados.
- `docs/`: auditoría, requisitos, arquitectura, guía de captura y [estado de la base operativa](docs/14-BASE-OPERATIVA.md).
- Las capturas visuales de referencia se conservan localmente y quedan fuera del repositorio público.
- `docs/diagramas/historicos/`: imágenes de diagramas anteriores, archivadas para evitar confundirlas con el diseño vigente.

El nombre visible del producto es **Cortex Go**; el identificador local del módulo Go es `cortex-go`. La reconstrucción contempla todos los módulos actuales excepto SocialStream Ninja, que el propietario indicó que no usa.

## Requisitos de desarrollo

El módulo apunta a Go 1.27.0 o posterior. En esta computadora quedó instalado Go 1.27.4. [Descargas oficiales de Go](https://go.dev/dl/).

## Iniciar Cortex local

El ejecutable abre el panel en el navegador predeterminado. Los datos de prueba se guardan en `data/first-cut.sqlite` y el registro en `data/logs/cortex-go.log`. Se puede cerrar Cortex desde el botón del panel; el servidor termina y cierra SQLite de forma ordenada.

Para compilar desde PowerShell en esta carpeta:

```powershell
New-Item -ItemType Directory -Force .\bin | Out-Null
go build -ldflags="-H=windowsgui" -o .\bin\CortexGo-Base.exe .\cmd\cortex-go
```

Después, iniciar `bin\CortexGo-Base.exe`. El ejecutable inicia un lanzador oculto que supervisa la aplicación principal y la recupera hasta cinco veces si termina con error. Si Cortex cierra normalmente, el lanzador también termina. Si se agota el límite o se cierra el propio lanzador, hay que iniciar el ejecutable otra vez. El ejecutable permite `--no-browser`, `--listen 127.0.0.1:17845` y `--data-dir <carpeta>` para pruebas controladas. El panel escucha solo en loopback; Tailscale Serve puede publicarlo únicamente dentro del tailnet. Cortex no tiene login propio: limita el tailnet a tus dispositivos de confianza.

Al iniciarse, Cortex aplica las migraciones de SQLite dentro de una transacción. La versión 1 copia los eventos sintéticos existentes al registro general sin perderlos y conserva backups antiguos importables. `POST /api/events` recibe eventos identificados por cuenta y directo; usa el ID de origen o una huella para deduplicar. El nombre y texto del comentario se guardan localmente hasta que se borren desde el historial. Borrar el historial activo no modifica las copias de seguridad anteriores.

Para este despliegue, la instancia disponible queda como host activo mientras la otra PC está apagada. La notebook accede al mismo panel por Tailscale; no mantengas dos instancias escribiendo la misma base ni sincronices el SQLite activo entre equipos. Cuando vuelva a estar disponible la PC habitual, se puede planificar el traslado con un respaldo verificado.

El panel también se puede instalar como aplicación PWA desde un navegador compatible cuando se abre por HTTPS, por ejemplo mediante Tailscale Serve. El manifiesto y el service worker permiten instalar la interfaz y guardar únicamente la carcasa estática para abrirla sin conexión; las API y los datos del panel nunca se guardan en la caché del navegador. En Chrome o Edge usa el botón **Instalar app** cuando aparezca o el control de instalación del navegador; en Safari de iOS, usa **Compartir → Añadir a pantalla de inicio**.

En el panel, **Respaldo local de SQLite** permite crear, restaurar y revisar los respaldos locales generados por Cortex Go; también permite borrar uno o todos con confirmación. Antes de restaurar, conserva automáticamente una copia de la base actual. Los respaldos no están cifrados ni se sincronizan. Borrar un comentario elimina la fila activa; si ya existe en respaldos, elimina también esos archivos desde la lista. Copias exportadas a otro lugar requieren su propio borrado.

La sección **Publicar código en GitHub** revisa los cambios de `main`, muestra el diff y solicita confirmación antes de crear un commit y hacer Push al remoto público `mati4315/Cortex-GO`. Solo permite revisar archivos Go, recursos concretos del panel PWA, fuentes del prototipo Facebook y Markdown de `docs/`; bloquea archivos preparados previamente, commits locales pendientes, ramas atrasadas y rutas fuera de esa lista. El botón no publica nada hasta que se confirme en el panel.

## Qué verifica y qué no

- El supervisor comprueba la salud del servidor interno cada segundo. Tras tres comprobaciones fallidas consecutivas lo reinicia; aplica esperas de 1, 2, 4 y 8 segundos, con hasta cinco reintentos automáticos antes de pedir intervención. El botón permite solicitar un reinicio manual.
- Si el ejecutable supervisor desaparece de forma abrupta, el servidor hijo se cierra al perder tres latidos, espaciados por tres segundos. No se instaló un servicio de Windows que vuelva a lanzar el propio ejecutable supervisor.
- Cuenta 1 y Cuenta 2 muestran el estado de los perfiles de captura. El API mantiene un endpoint sintético para desarrollo; solo el prototipo separado envía comentarios de Facebook después de la autenticación manual. No se importan cuentas, credenciales ni datos del Cortex anterior.
- OBS usa el WebSocket local (por defecto `ws://127.0.0.1:4455`) para completar la identificación de sesión. Solo acepta direcciones de loopback; no solicita escenas ni envía órdenes. Si OBS requiere clave, se muestra el estado y no se guarda esa clave.
- “Destacar” solo cambia el aspecto de la fila en este panel de prueba; todavía no activa una escena ni una fuente de OBS.

## Desarrollo

Se requiere Go 1.27.0 o posterior. Las dependencias directas son `modernc.org/sqlite` (SQLite sin CGO) y `github.com/gorilla/websocket`.

Para comprobar ambos módulos:

```powershell
go test ./...
Push-Location .\experiments\facebook-browser-poc
go test ./...
Pop-Location
```

Para verificar el código y compilar desde PowerShell:

```powershell
go test ./...
go build -ldflags="-H=windowsgui" -o .\bin\CortexGo-Base.exe .\cmd\cortex-go
```

## Documentación

Empieza por [`docs/00-INDICE.md`](docs/00-INDICE.md), consulta el [estado de la base operativa](docs/14-BASE-OPERATIVA.md), la [guía Facebook](docs/13-GUIA-CAPTURA-COMENTARIOS-FACEBOOK.md) y el [plan para PC y notebook](docs/12-SINCRONIZACION-ENTRE-PC.md). El primer corte técnico queda como antecedente en `docs/11-PRIMER-CORTE-PRUEBA-TECNICA.md`.
