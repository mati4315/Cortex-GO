# Sincronización de desarrollo entre la PC y la notebook

**Estado:** el propietario confirmó que el repositorio `mati4315/Cortex-GO` seguirá público. GitHub contendrá solo código, documentación y recursos revisados que no incluyan datos privados o delicados. La sincronización privada con la notebook es un flujo separado y optativo.  
**Situación observada:** Git 2.56.0 está instalado. El repositorio local `main` ya fue inicializado y sus archivos revisados están preparados para el primer commit. El remoto está vacío; todavía no se hizo commit ni se envió contenido. Las capturas con personas, nombres y comentarios permanecen en la PC y están excluidas del índice de Git.
**Objetivo:** poder seguir desarrollando el mismo proyecto nuevo desde cualquiera de las dos computadoras.

## Recomendación

Usar **GitHub público con GitHub Desktop** para el código y la documentación aptos para publicación. El remoto conserva el historial de cambios y permite descargar una copia del proyecto en la notebook. Antes del primer push se excluyen las capturas personales, secretos, bases, archivos locales y cualquier ruta o identificador de máquina que no haga falta.

GitHub Desktop permite agregar la carpeta existente, revisar archivos, crear commits, publicar un repositorio y hacer Push/Pull desde botones. No hace falta mantener la carpeta del proyecto dentro de OneDrive o Dropbox.

### Rutina simple

1. Al terminar en la PC principal, revisar los cambios en GitHub Desktop, escribir un mensaje breve, crear un commit y pulsar **Push origin**.
2. En la notebook, clonar el remoto una sola vez. Antes de empezar cada sesión, pulsar **Fetch/Pull origin** para bajar los últimos commits.
3. Al terminar en la notebook, crear un commit y pulsar **Push origin**.
4. Antes de volver a editar en la PC principal, hacer **Pull origin**.

Un commit es el punto guardado local; Push lo publica en el remoto y Pull baja los cambios. Cambios no guardados en un commit todavía no aparecen en la otra computadora. Conviene editar en una sola PC a la vez y sincronizar al cambiar de equipo para evitar conflictos.

## Qué entra y qué no entra al repositorio

**Mantener en Git público:** código fuente, pruebas, documentación ya revisada, diagramas genéricos, `go.mod`, `go.sum`, `.gitignore` y ejemplos de configuración sin secretos.

**No subir al remoto público:** `.cache`, `bin`, ejecutables `.exe`, carpetas `data` y `logs`, bases de datos o sus copias, credenciales, tokens, archivos `.env` reales, capturas con datos personales ni configuraciones locales. El `.gitignore` excluye estas rutas y las capturas de referencia. Go reconstruye el ejecutable en cada computadora y vuelve a descargar dependencias desde `go.mod`/`go.sum`.

Esto sincroniza todo lo necesario para continuar programando, pero no copia cada archivo generado mientras la aplicación se ejecuta. Si más adelante se decide conservar historial de datos entre equipos, diseñar una copia de seguridad exportable; no sincronizar en vivo un mismo archivo SQLite mediante OneDrive/Dropbox ni abrirlo como escritor en ambas PCs.

## Archivos privados y base de datos entre las dos PCs

Para la copia privada, usar **Syncthing** en una carpeta separada, por ejemplo `Cortex-Privado`, configurada únicamente entre la PC de escritorio y la notebook. El propietario quiere que Cortex pregunte al cerrar o al solicitar una copia: **“¿Sincronizar la copia privada con la notebook?”** Si acepta, Cortex prepara y deja el paquete en esa carpeta; Syncthing lo transfiere cuando ambos equipos están encendidos y disponibles. Si la notebook está apagada, la transferencia queda pendiente hasta que vuelva a conectarse. El tráfico entre dispositivos usa TLS y cada dispositivo debe ser aprobado por su identidad. Habilitar versionado de archivos. No sincronizar la carpeta `.git` ni el proyecto entero por Syncthing.

La base SQLite activa requiere otro tratamiento. No sincronizar directamente el archivo mientras Cortex lo está usando: en modo WAL hay archivos auxiliares y restricciones que hacen insegura la sincronización de la base activa o su uso desde una carpeta de red. Cuando se implemente la base real, Cortex deberá crear un snapshot consistente mediante la API de backup de SQLite y cifrar el paquete antes de colocarlo en la carpeta sincronizada. La notebook recibirá una copia fechada para desarrollo, consulta o recuperación; la base activa seguirá en la PC de escritorio. Si se necesita control o acceso en vivo desde la notebook, usar una API de Cortex autenticada en la PC de escritorio.

Las claves que el propietario autorice a tener en ambos equipos no se guardarán en texto plano dentro de la carpeta sincronizada. Mantenerlas en una bóveda cifrada que se sincroniza como archivo cifrado y pedir la contraseña maestra localmente en cada PC; no guardar esa contraseña, claves de descifrado ni cookies/sesiones de Facebook en Git o en la misma carpeta. Proteger también los discos de ambos equipos con cifrado del sistema. El prototipo actual solo usa una base sintética de desarrollo, ignorada por Git; el paquete cifrado y la confirmación de usuario se implementarán antes de usar datos reales.

Para videos, bibliotecas multimedia u otros assets grandes, evaluar almacenamiento compartido o Git LFS según tamaño y frecuencia. No llenar el historial de Git con bases activas, logs o archivos que se regeneran.

## Alternativas

| Opción | Ventaja | Límite | Recomendación |
|---|---|---|---|
| GitHub público + GitHub Desktop | Historial y flujo visual sencillo de Push/Pull para código y documentos revisados. | Todo lo que entra al remoto puede verlo cualquiera. | Mejor equilibrio para el código público; aplicar la lista de exclusión. |
| GitLab privado + Git/cliente visual | Mismo modelo de remoto privado e historial. | La configuración inicial puede ser menos directa si se busca usar GitHub Desktop. | Buena alternativa si ya usás GitLab. |
| Trabajar en la PC por acceso remoto desde la notebook | Una sola carpeta de trabajo; no hay copias locales que reconciliar. | La PC principal debe estar encendida y accesible. | Útil para una sesión puntual, no reemplaza un respaldo del código. |
| Sincronización de carpeta (OneDrive/Dropbox/Syncthing) | Copia archivos automáticamente. | No entiende commits/ramas; puede crear conflictos al sincronizar `.git` o una base activa. | Usar una carpeta separada para el paquete privado optativo. |
| Syncthing + bóveda y backup cifrados | Copia privada entre las dos PCs, con transferencia TLS; puede esperar a que la notebook vuelva a estar en línea. | Cada PC confiable conserva acceso a los archivos que descifre. | Candidato elegido para el paquete privado, con confirmación del usuario y versionado. |
| Copia manual en USB | Funciona sin cuenta ni red. | Fácil llevar una versión vieja o pisar cambios. | Solo respaldo adicional de emergencia. |

## Orden seguro de puesta en marcha

1. Revisar la lista pública y excluir capturas, datos personales, rutas locales, `.env`, bases, logs y ejecutables.
2. Revisar `.gitignore` y configurar el nombre/correo de autor de Git para este repositorio.
3. Crear el primer commit local y publicarlo; verificar que no haya archivos delicados.
4. En la notebook, instalar Go compatible, clonar el remoto y correr `go test ./...` y `go build`.
5. Probar el ciclo PC → notebook → PC con un cambio inocuo en un documento antes de empezar a trabajar alternadamente en módulos.

La pregunta de sincronización privada debe describir qué se enviará y pedir confirmación explícita. El paquete incluirá solo archivos autorizados, una copia consistente de SQLite cifrada y la bóveda cifrada de credenciales; nunca incluirá sesiones/cookies del navegador por defecto. Mostrar progreso, fecha del último envío y si la notebook está desconectada. Las credenciales se desbloquean localmente; el paquete no incluirá la contraseña maestra.

## Fuentes oficiales

- [GitHub: repositorios privados](https://docs.github.com/en/repositories/creating-and-managing-repositories/about-repositories)
- [GitHub Desktop: agregar y clonar repositorios](https://docs.github.com/en/desktop/adding-and-cloning-repositories)
- [GitHub Desktop: sincronizar ramas con Push/Pull](https://docs.github.com/en/desktop/working-with-your-remote-repository-on-github-or-github-enterprise/syncing-your-branch-in-github-desktop)
- [Git: clonar un repositorio](https://git-scm.com/docs/git-clone)
- [Syncthing: principios de seguridad](https://docs.syncthing.net/users/security.html)
- [Syncthing: versionado de archivos](https://docs.syncthing.net/users/versioning.html)
- [SQLite: limitaciones de WAL en sistemas de archivos de red](https://www.sqlite.org/wal.html)
- [SQLite: API oficial de backup en línea](https://www.sqlite.org/backup.html)
