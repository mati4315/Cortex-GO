# Cortex Go

Reconstrucción nueva de Cortex. El código anterior se conserva solo como referencia funcional: no se copia ni se migra.

## Estado

El primer corte técnico ya está implementado como prueba local: un ejecutable inicia el panel central y supervisa un servidor interno; hay dos feeds simulados, un registro sintético en SQLite y una comprobación WebSocket de OBS de solo lectura. Esto todavía no conecta Facebook ni reemplaza módulos del Cortex anterior.

## Estructura

- `cmd/cortex-go/`: punto de entrada del ejecutable.
- `cmd/cortex-go/web/index.html`: panel central incluido dentro del ejecutable.
- `docs/`: auditoría, requisitos, arquitectura y criterios del primer corte.
- Las capturas visuales de referencia se conservan localmente y quedan fuera del repositorio público.
- `docs/diagramas/historicos/`: imágenes de diagramas anteriores, archivadas para evitar confundirlas con el diseño vigente.

El nombre visible del producto es **Cortex Go**; el identificador local del módulo Go es `cortex-go`. La reconstrucción contempla todos los módulos actuales excepto SocialStream Ninja, que el propietario indicó que no usa.

## Requisitos de desarrollo

El módulo apunta a Go 1.27.0 o posterior. En esta computadora quedó instalado Go 1.27.4. [Descargas oficiales de Go](https://go.dev/dl/).

## Ejecutar la prueba local

El ejecutable abre el panel en el navegador predeterminado. Los datos de prueba se guardan en `data/first-cut.sqlite` y el registro en `data/logs/cortex-go.log`. Se puede cerrar Cortex desde el botón del panel; el servidor termina y cierra SQLite de forma ordenada.

Para compilar desde PowerShell en esta carpeta:

```powershell
New-Item -ItemType Directory -Force .\bin | Out-Null
go build -ldflags="-H=windowsgui" -o .\bin\CortexGo.exe .\cmd\cortex-go
```

Después, iniciar `bin\CortexGo.exe`. El ejecutable permite `--no-browser`, `--listen 127.0.0.1:17845` y `--data-dir <carpeta>` para pruebas controladas. El panel se limita a loopback y no está expuesto a la notebook ni a la red local.

En el panel, la sección **Respaldo local de SQLite** permite crear un respaldo en `<carpeta de datos>\backups` o elegir un archivo `.sqlite`/`.db` para restaurarlo. Antes de restaurar, Cortex Go conserva automáticamente una copia de la base actual en esa misma carpeta. Esta función solo administra la base sintética de prueba; todavía no cifra ni sincroniza archivos con la notebook.

La sección **Publicar código en GitHub** revisa los cambios de `main`, muestra el diff y solicita confirmación antes de crear un commit y hacer Push al remoto público `mati4315/Cortex-GO`. Solo permite revisar `README.md`, `go.mod`, `go.sum`, código Go/HTML de `cmd/cortex-go/` y Markdown de `docs/`; bloquea archivos preparados previamente, commits locales pendientes, ramas atrasadas y tipos de archivo fuera de esa lista. El botón no publica nada hasta que se confirme en el panel.

## Qué verifica y qué no

- El supervisor comprueba la salud del servidor interno cada segundo. Tras tres comprobaciones fallidas consecutivas lo reinicia; aplica esperas de 1, 2, 4 y 8 segundos, con hasta cinco reintentos automáticos antes de pedir intervención. El botón permite solicitar un reinicio manual.
- Si el ejecutable supervisor desaparece de forma abrupta, el servidor hijo se cierra al perder tres latidos, espaciados por tres segundos. No se instaló un servicio de Windows que vuelva a lanzar el propio ejecutable supervisor.
- Feed A y Feed B son simulaciones. Los comentarios ingresados en el panel se guardan en la nueva SQLite de prueba; no se consultan cuentas, credenciales ni base de datos anterior.
- OBS usa el WebSocket local (por defecto `ws://127.0.0.1:4455`) para completar la identificación de sesión. Solo acepta direcciones de loopback; no solicita escenas ni envía órdenes. Si OBS requiere clave, se muestra el estado y no se guarda esa clave.
- “Destacar” solo cambia el aspecto de la fila en este panel de prueba; todavía no activa una escena ni una fuente de OBS.

## Desarrollo

Se requiere Go 1.27.0 o posterior. Las dependencias directas son `modernc.org/sqlite` (SQLite sin CGO) y `github.com/gorilla/websocket`.

Para verificar el código y compilar desde PowerShell:

```powershell
go test ./...
go build -ldflags="-H=windowsgui" -o .\bin\CortexGo.exe .\cmd\cortex-go
```

## Documentación

Empieza por [`docs/00-INDICE.md`](docs/00-INDICE.md), sigue con [`docs/11-PRIMER-CORTE-PRUEBA-TECNICA.md`](docs/11-PRIMER-CORTE-PRUEBA-TECNICA.md) y consulta [`docs/12-SINCRONIZACION-ENTRE-PC.md`](docs/12-SINCRONIZACION-ENTRE-PC.md) para trabajar entre la PC y la notebook.
