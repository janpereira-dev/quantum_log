# Uso local de Codex, cuando lo necesitas

Quantum Log importa archivos de sesión de Codex que ya existen en tu equipo,
guarda sus datos de uso en un registro local y muestra los tokens por proyecto.
También permite exportar un CSV. Cada comando termina al completar su trabajo;
no necesita un proceso en segundo plano.

[English guide](../SIMPLE.md) · [Índice de documentación](README.md)

## Compilar

Necesitas Git y la versión de Go indicada en `go.mod`. En PowerShell:

```powershell
git clone https://github.com/janpereira-dev/quantum_log.git
cd quantum_log
go build -o qlog.exe ./cmd/qlog
```

En Linux o macOS, compila con `go build -o qlog ./cmd/qlog` y sustituye
`.\qlog.exe` por `./qlog` en los ejemplos.

`import` crea su registro automáticamente cuando encuentra archivos para
importar. No hay un paso de inicialización ni configuración de agentes.
Ejecutar `qlog` equivale a `qlog report`; antes de importar muestra un informe
vacío. `qlog --help` enumera los comandos disponibles.

## Probar con datos ficticios

El repositorio incluye [una sesión de demostración](../../fixtures/demo-session.jsonl).
Desde la raíz del repositorio, prueba con un registro separado:

```powershell
.\qlog.exe --home .qlog/demo import fixtures/demo-session.jsonl
.\qlog.exe --home .qlog/demo report
.\qlog.exe --home .qlog/demo export > demo-usage.csv
```

El resultado muestra dos muestras de uso y **150 tokens**, el proveedor
`openai` y el modelo `demo-model`. El proyecto figura como `unattributed`
porque la sesión ficticia no contiene un directorio real de proyecto. Los
contadores acumulados son 100 y 150: el total es 150. Importar de nuevo mantiene
ese total. Los costes figuran como no disponibles.

Los nombres y cifras son ficticios. Estos comandos no leen tus sesiones de
Codex ni usan tu registro habitual.

## Descargas portátiles

Cuando una release incluya archivos `qlog-simple_VERSION_OS_ARCH`, descarga el
que corresponda a tu sistema operativo y arquitectura, junto con el
`checksums.txt` de esa misma release. Antes de extraerlo, compara su SHA-256
con la entrada exacta del archivo en `checksums.txt`.

En Windows puedes consultar el hash con
`Get-FileHash ARCHIVO.zip -Algorithm SHA256`; en Linux, con
`sha256sum ARCHIVO.tar.gz`; y en macOS, con `shasum -a 256 ARCHIVO.tar.gz`.

El ejecutable se llama `qlog-simple` (`qlog-simple.exe` en Windows). Usa ese
nombre en los ejemplos de esta guía. Los archivos `qlog_VERSION_OS_ARCH` y
los instaladores existentes corresponden al producto anterior. La presencia
de estas instrucciones no significa que se haya publicado una nueva release.

## Elegir qué importar

Para importar tus propias sesiones y consultar el resultado:

```powershell
.\qlog.exe import
.\qlog.exe report
.\qlog.exe export > usage.csv
```

Sin una ruta explícita, Quantum Log busca en `$CODEX_HOME/sessions` si has
definido `CODEX_HOME`; de lo contrario, usa `~/.codex/sessions`. Lee los
archivos de origen sin modificarlos.

```powershell
.\qlog.exe import
.\qlog.exe import --days 30 --limit 100
.\qlog.exe import C:\ruta\sesion.jsonl
.\qlog.exe import --days 30 C:\ruta\sesiones
.\qlog.exe import --json
```

Coloca las opciones antes de la ruta. Al recorrer una carpeta, selecciona
primero los archivos más recientes: por defecto, los modificados en los
últimos siete días, con un máximo de 100 por ejecución. `--days` acepta
1–36500 y `--limit` acepta 1–100. Un archivo indicado expresamente se importa
independientemente de su antigüedad.

El límite restringe los archivos importados, no el recorrido de carpetas ni
el tamaño de cada archivo. Un archivo recién modificado puede contener
eventos antiguos. Importar de nuevo actualiza la información y evita duplicar
la evidencia ya guardada.

## Leer y exportar

| Comando | Resultado |
| --- | --- |
| `qlog` o `qlog report` | Resumen de todo lo importado |
| `qlog report --today` | Uso del día natural actual, según la hora local |
| `qlog report --project PROYECTO` | Resumen del identificador de proyecto indicado |
| `qlog report --json` | Informe en JSON |
| `qlog export > usage.csv` | CSV con datos agregados |
| `qlog export --today --project PROYECTO > today.csv` | CSV filtrado por día y proyecto |

Usa el identificador de proyecto que aparece en el informe. Los resultados
mantienen proyecto, agente, proveedor, modelo y calidad de los datos. El CSV
sale por la salida estándar y omite las rutas de origen y el contenido de
conversaciones.

Los tokens proceden de contadores acumulados registrados por Codex, con
calidad `agent_reported`. Las muestras de uso no son un recuento de llamadas
al modelo ni garantizan que todas las llamadas aparezcan en los archivos.
Los costes y el gasto de la suscripción **no están disponibles**; no se
interpretan como cero.

En sesiones creadas por otros agentes o mediante una bifurcación se excluyen
los contadores anteriores a su creación. Si el límite de creación o el
primer contador no permiten atribuir el uso con seguridad, se omiten esas
muestras. El resultado de importación muestra cuántas se han omitido.

## Almacenamiento y privacidad

El registro actual usa una carpeta `simple` separada de la versión anterior:

| Sistema | Carpeta predeterminada si no has definido `QLOG_HOME` |
| --- | --- |
| Windows | `%LOCALAPPDATA%\QUANTUM_LOG\simple` |
| Linux | `$XDG_DATA_HOME/quantum-log/simple` o `~/.local/share/quantum-log/simple` |
| macOS | `~/Library/Application Support/QUANTUM_LOG/simple` |

Con `QLOG_HOME` definido, usa `$QLOG_HOME/simple`. `--home RUTA` selecciona
directamente otra carpeta: puede ser una carpeta nueva para una prueba
aislada o, de forma deliberada, un registro existente. El registro
predeterminado anterior permanece separado; no se selecciona ni migra por defecto.

El importador conserva metadatos seleccionados y contadores de uso. No guarda
prompts, respuestas, argumentos o resultados de herramientas ni contenido de
archivos. Los metadatos locales siguen identificando sesiones y proyectos;
protege esa carpeta y revisa los nombres de proyecto antes de compartir un CSV.

Si eliges un registro antiguo que requiere actualizar el esquema, haz primero
una copia y ejecuta la operación explícita de mantenimiento:

```powershell
.\qlog.exe --home C:\ruta\registro-anterior migrate
.\qlog.exe --home C:\ruta\registro-anterior report
```

`migrate` conserva la evidencia de uso y no necesita los archivos de sesión
originales. `report` y `export` no actualizan el esquema por su cuenta.

## Límites

- Solo se admiten los archivos locales de Codex disponibles. Pueden estar
  incompletos, faltar o cambiar de formato.
- La importación es manual. No configura agentes, servicios, tareas de
  inicio, hooks, captura en tiempo real ni un servidor MCP.
- Crear o usar este ejecutable no modifica una instalación anterior. Un
  collector antiguo debe gestionarse con los controles de aquella versión.
- Las mediciones de recursos deben identificar la compilación, el volumen
  de origen y el equipo utilizado. El [informe local fechado](../verification/simple-local-ledger.md)
  conserva sus resultados y sus limitaciones; no verifica cualquier nueva compilación.

La [documentación histórica](../legacy/README.md) conserva las instrucciones
del producto anterior. Para contribuir al actual, consulta
[CONTRIBUTING.md](../../CONTRIBUTING.md).
