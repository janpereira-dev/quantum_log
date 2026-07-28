# Operaciones

Opera QUANTUM_LOG mediante ciclo de vida CLI y bloqueos cooperativos. Mantén libro mayor local, verifícalo antes de confiar en reportes y trata acciones de mantenimiento no disponibles como límites, no invitaciones a improvisar.

[Read in English](../en/operations.md)

## Comprobaciones rutinarias

```bash
qlog doctor --json
qlog verify
qlog collector status --json
qlog adapter status --json
```

`doctor` comprueba SQLite y estado de libro mayor local sin mutación. `verify` comprueba integridad de cadena append-only. `collector status` informa endpoints y alcance. `adapter status` informa estado de instalación y calidad de captura, no afirmaciones de uso verificado.

Ejecuta diagnósticos cuando writers normales estén inactivos. Diagnósticos adquieren acceso exclusivo de quiescencia, así que bloqueo retenido o WAL activo puede hacerlos fallar. Primero detén o termina actividad qlog relevante; no fuerces apertura externa de SQLite.

## Operación de collector

Endpoint de collector por defecto es `http://127.0.0.1:4318`:

```bash
qlog collector serve
qlog collector status
qlog collector logs
```

Endpoints son:

| Endpoint | Entrada |
| --- | --- |
| `/v1/traces` | Trazas OTLP/HTTP JSON o protobuf. |
| `/v1/events` | Eventos qlog JSON. |
| `/healthz` | Comprobación de estado `GET` o `HEAD`. |

Para ciclo de vida de servicio gestionado usa `install`, `start`, `stop`, `restart`, `logs` y `uninstall`. Listener fuera de loopback se rechaza salvo que `--allow-non-loopback` sea explícito. Si haces opt-in, asumes consecuencias de exposición de red, firewall, autenticación y hardening del host.

## Operación de adaptador

Comienza con descubrimiento y dry run:

```bash
qlog adapter detect
qlog setup --dry-run
qlog adapter install opencode --dry-run
```

Luego aplica y prueba adaptador específico:

```bash
qlog setup opencode --yes
qlog adapter test opencode
qlog adapter status opencode
```

Usa `qlog adapter uninstall <adapter> --dry-run` antes de eliminar. Configuración de adaptador solo debería tocar configuración propiedad de qlog, pero revisa cambios planificados y backups antes de aplicarlos.

### Verificación de Copilot

M4 está `IN_PROGRESS`. Copilot VS Code sigue experimental hasta que trace real persista tokens originados en Copilot. Usa:

```bash
qlog adapter verify copilot-vscode --project my-project --since 1h --json
```

Esto verifica ajustes, alcance de collector, duración válida, acceso a base de datos local y model call OTLP calificado reciente de Copilot. Extensión configurada sin evidencia persistida de tokens no es ruta de captura verificada.

## Límites de respaldo y recuperación

Antes de copiar libro mayor, detén clientes qlog y ejecuta:

```bash
qlog maintenance checkpoint
qlog verify
qlog anchor export > anchors.json
```

Almacena backup resultante y `anchors.json` por separado. Anchors externos son valiosos porque permiten que comprobación posterior detecte desajuste o truncamiento:

```bash
qlog anchor check --file anchors.json
```

`maintenance recover` y `maintenance rebuild-anchor` están bloqueados intencionalmente mientras se completa trabajo de anchors. No afirmes que existe procedimiento de recuperación soportado, ni reconstruyas o edites estado de libro mayor con herramienta SQLite externa. Escala incidente de libro mayor dañado con archivos originales, salida de comandos y anchors externos intactos.

## Fallos de bloqueo y WAL

| Síntoma | Significado | Respuesta segura |
| --- | --- | --- |
| `quiescence lock is held` | Otro cliente oficial está activo. | Termina o detén ese cliente y reintenta diagnóstico. |
| Fallo de WAL activo | Diagnóstico estable de solo lectura no puede continuar con seguridad. | Deja que writer cierre o usa ruta checkpoint soportada después de quiescencia. |
| Migración pendiente | Esquema de libro mayor actual no coincide con aplicación. | Ejecuta `qlog init` con versión actual de qlog después de cumplir política de backup. |
| Advertencia SHM aislada | Sidecar SQLite necesita atención de operador. | Preserva evidencia; inspecciona con ciclo de vida qlog soportado. |

## Operaciones de evidencia

Usa reportes y exportaciones con procedencia:

```bash
qlog usage today --group-by project,agent,provider,model,capture_quality
qlog report --json
qlog export --format csv --redact-paths > usage.csv
```

Antes de compartir, usa `--redact-paths` cuando rutas de ubicación no sean necesarias. No elimines `capture_quality` al agregar resultados para comparar integraciones o producir declaración de auditoría.

## Entrega de incidente

Captura estos datos antes de escalar:

1. Versión qlog y sistema operativo.
2. Comando exacto, flags, estado de salida, stdout y stderr.
3. Si proceso de collector o adaptador estaba en ejecución.
4. Salida de `qlog doctor --json` y `qlog verify`, si estado de bloqueo lo permite.
5. Si existe anchor externo y resultado de `qlog anchor check --file ...`.

No adjuntes prompts raw, respuestas, payloads de herramientas, API keys ni contenido copiado de base de datos a incidente por defecto. Ver [Privacidad y seguridad](privacidad-seguridad.md).
