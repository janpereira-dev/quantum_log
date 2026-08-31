# Guía

Esta guía lleva desde un libro mayor local vacío hasta evidencia de uso verificada y explícitamente calificada. QUANTUM_LOG prioriza lo local: no requiere cuenta SaaS, proxy ni archivo de prompts.

[Read in English](../en/guide.md)

## Camino rápido

```bash
# Primero instala un binario publicado siguiendo ../INSTALL.md.
qlog init
qlog project register --path . --name MY_PROJECT
qlog project current --json
qlog verify
qlog doctor --json
```

`qlog init` crea configuración local y libro mayor. `qlog project register` registra un proyecto lógico y su ubicación. `qlog verify` comprueba cadenas de eventos append-only. `qlog doctor` comprueba estado del libro mayor local sin modificarlo.

## Instalación

Instala un binario publicado y verificado siguiendo [Install](../INSTALL.md).
No uses `go install` como canal de distribución para usuarios: evita el archivo
de release y su verificación de checksum.

Para desarrollo, compila desde este checkout:

```bash
go build -o qlog ./cmd/qlog
./qlog --help
```

Cada comando acepta `--home <path>` para sobrescribir directorio local de datos de QUANTUM_LOG. Usa un home explícito para pruebas aisladas o automatización.

## Inicializar y registrar

```bash
qlog init
qlog project register --path . --name MY_PROJECT
qlog project current --json
```

Salida esperada incluye `initialized QUANTUM_LOG at ...`, seguido de `registered my-project at ...`. JSON de `project current` informa slug de proyecto, método de resolución, confianza y ubicación cuando resuelve uno.

Atribución de proyecto se basa en evidencia. Resolución comprueba `--project` explícito, `QLOG_PROJECT`, directorio actual, raíz Git, ruta registrada, señal del adaptador y luego deja datos sin atribuir. Nunca adivina propiedad desde proveedor, modelo o nombre de agente. Ver [Arquitectura](arquitectura.md).

## Ingerir eventos normalizados

Importa JSON delimitado por nueva línea desde archivo o entrada estándar:

```bash
qlog ingest file events.ndjson
cat events.ndjson | qlog ingest stdin
```

Importaciones exitosas informan `imported N event(s)`. Entrada se normaliza y sanitiza antes de almacenamiento y hashing. No trates campos de tokens importados como autoritativos salvo que su `capture_quality` respalde esa afirmación.

## Configuración de captura

Primero inspecciona integraciones instaladas:

```bash
qlog adapter list
qlog adapter detect
qlog adapter status
qlog setup --dry-run
```

Aplica integración específica solo tras revisar plan de dry-run:

```bash
qlog setup opencode --dry-run
qlog setup opencode --yes
qlog adapter test opencode
```

Usa `qlog collector serve` cuando adaptador envía payloads OTLP o qlog event al collector local. Collector escucha en loopback por defecto. Vincular dirección fuera de loopback requiere opt-in explícito `--allow-non-loopback`.

Para hooks de Claude Code, configura host para pasar JSON del hook a:

```bash
qlog hook claude-code
```

Captura con hook y wrapper puede ser solo de ciclo de vida. Evidencia de ciclo de vida registra que proceso o sesión existió; no es uso de tokens.

### Límite de evidencia de Copilot

M4 está `IN_PROGRESS`. Configuración de Copilot VS Code e ingestión OTLP siguen experimentales hasta que trace real originado en Copilot persista uso de tokens en SQLite. Ajustes instalados por sí solos no verifican captura. Usa:

```bash
qlog adapter verify copilot-vscode --project my-project --since 1h --json
```

`qlog adapter verify copilot-vscode` está listo solo cuando ajustes están instalados, collector es alcanzable y existe evidencia reciente de model-call originada en Copilot con `otel_reported` y tokens. Ninguna etapa interna única prueba captura. No afirmes captura de tokens de Copilot desde dry run, resultado de instalación o evento genérico importado.

## Consultar uso y costo

```bash
qlog usage today
qlog usage project my-project --json
qlog report --from 2026-07-01 --to 2026-08-01
```

Salida de uso incluye calidad de captura. Es contexto obligatorio: `otel_reported`, `agent_reported`, `lifecycle_only`, `unavailable` y otras etiquetas no son equivalentes. QUANTUM_LOG no inventa conteos de tokens.

Agrega reglas de precios versionadas antes de esperar costo calculado:

```bash
qlog pricing validate pricing.json
qlog pricing add pricing.json
qlog pricing recalculate --from 2026-07-01 --to 2026-08-01
```

Luego inspecciona o repara asignaciones explícitas de costo solo cuando conoces proyecto correcto:

```bash
qlog allocation show <model-call-id>
qlog allocation repair <model-call-id> --project my-project
```

## Verificar y solucionar problemas

Ejecuta estos comandos cuando no haya writer activo:

```bash
qlog verify
qlog doctor --json
qlog maintenance status
qlog collector status
```

`verify` comprueba cadenas hash de source/session. `doctor` realiza comprobación de estado de solo lectura. Ambos toman bloqueo diagnóstico exclusivo y pueden fallar mientras otro cliente oficial mantiene bloqueo cooperativo de quiescencia o existe WAL activo. Es protección, no razón para abrir base de datos con editor SQLite externo.

Usa `qlog maintenance checkpoint` solo después de detener clientes oficiales. `maintenance recover` y `maintenance rebuild-anchor` no están disponibles intencionalmente y devuelven error not-implemented.

## Referencias siguientes

- [Referencia CLI](referencia-cli.md): sintaxis, flags, ejemplos y estados de fallo.
- [Arquitectura](arquitectura.md): capas, resolución, ciclo de vida de eventos y bloqueos.
- [Operaciones](operaciones.md): diagnósticos, respaldo, límites de recuperación y anchors.
- [Privacidad y seguridad](privacidad-seguridad.md): sanitización, política de calidad de captura y modelo de amenazas.
- [Contribución](contribucion.md): desarrollo local y entrega de releases.
