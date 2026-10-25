# Quantum Log — portable / portátil

Import local Codex session files, view reported tokens, and export aggregate CSV.
No Go installation or background process is required.

Importa sesiones locales de Codex, consulta los tokens registrados y exporta CSV.
No necesitas instalar Go ni mantener un proceso en segundo plano.

Verify the archive's SHA-256 against its release's `checksums.txt` before use.
Comprueba el SHA-256 del archivo con el `checksums.txt` de su misma release.

## 1. Try the demo / Prueba la demostración

Extract the archive into a new folder and open a terminal there.
Extrae el archivo en una carpeta nueva y abre una terminal en esa carpeta.

Windows — PowerShell:

```powershell
.\qlog-simple.exe --home .qlog/demo import fixtures/demo-session.jsonl
.\qlog-simple.exe --home .qlog/demo report
```

Linux / macOS:

```sh
./qlog-simple --home .qlog/demo import fixtures/demo-session.jsonl
./qlog-simple --home .qlog/demo report
```

Expected: **2 usage samples, 150 tokens, cost unavailable**. The project is
`unattributed`; the model is `demo-model`. These are synthetic data. Reimporting
keeps the total at 150. The demo uses its own `.qlog/demo` ledger and does not
read your personal Codex sessions.

Resultado: **2 muestras de uso, 150 tokens y coste no disponible**. El proyecto
es `unattributed` y el modelo, `demo-model`. Son datos ficticios. Volver a importar
mantiene el total en 150. La prueba usa su propio registro `.qlog/demo` y no lee
tus sesiones personales de Codex.

## 2. Import your sessions / Importa tus sesiones

Windows — PowerShell:

```powershell
.\qlog-simple.exe import
.\qlog-simple.exe report
.\qlog-simple.exe export > usage.csv
```

Linux / macOS:

```sh
./qlog-simple import
./qlog-simple report
./qlog-simple export > usage.csv
```

Import reads existing Codex files, newest first: by default, up to 100 files
modified in the last 7 days. Import again to refresh the report. An explicit
file can be older: `import path/to/session.jsonl`. Use `--help` for filters.

La importación lee archivos existentes de Codex, empezando por los más recientes:
por defecto, hasta 100 modificados en los últimos 7 días. Importa de nuevo para
actualizar el informe. Puedes indicar un archivo antiguo con
`import ruta/sesion.jsonl`. Consulta `--help` para ver los filtros.

## Data and limits / Datos y límites

Each command exits when finished. Default storage uses a separate `simple`
directory inside Quantum Log's local data directory; `--home` selects an exact
directory instead. Source files are unchanged. Prompts, responses, and tool
content are not retained. Exports omit source paths; project names and other
metadata can still be private. Usage samples are not model-call counts. Costs
and subscription spend are unavailable. Only available local Codex logs are
supported; missing or incomplete logs limit the report.

Cada comando termina al acabar. El registro predeterminado usa una carpeta
`simple` separada dentro del directorio local de Quantum Log; `--home` selecciona
otra carpeta exacta. Los archivos originales no se modifican. No se guardan
prompts, respuestas ni contenido de herramientas. El CSV omite rutas, pero los
nombres de proyecto y otros metadatos pueden ser privados. Las muestras no son
un recuento de llamadas al modelo. Los costes y el gasto de la suscripción no
están disponibles. Solo se admiten logs locales de Codex; si faltan o están
incompletos, el informe también lo estará.

[Full English guide](https://github.com/janpereira-dev/quantum_log/blob/main/docs/SIMPLE.md)
· [Guía completa en español](https://github.com/janpereira-dev/quantum_log/blob/main/docs/es/simple.md)

MIT license / Licencia MIT: see / consulta `LICENSE`.
